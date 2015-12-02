package voip

import (
	"errors"
	"fmt"
	"log"

	"github.com/Unknwon/goconfig"
	docker "github.com/mangalaman93/dockerclient"
	"github.com/satori/go.uuid"
)

const (
	IMG_SIPP       = "mangalaman93/sipp"
	IMG_SNORT      = "mangalaman93/snort"
	IMG_CADVISOR   = "mangalaman93/cadvisor"
	IMG_MONCONT    = "mangalaman93/moncont"
	SIPP_BUFF_SIZE = "1048576"
	CPU_PERIOD     = 100000
	STOP_TIMEOUT   = 5
	BUF_DURATION   = "5s"
)

var (
	ErrHostNotFound = errors.New("Host not found")
	ErrNoHosts      = errors.New("error while finding host list")
)

type DockerCManager struct {
	dockercls map[string]*docker.DockerClient
	hmap      map[string]string
	cadvisor  []string
	moncont   []string
}

func NewDockerCManager(config *goconfig.ConfigFile) (*DockerCManager, error) {
	hosts := config.GetKeyList("VOIP.TOPO")
	if hosts == nil {
		return nil, ErrNoHosts
	}

	var iuser, ipass string
	var err error
	if s, _ := config.GetSection("VOIP.DB"); s != nil {
		iuser, err = config.GetValue("VOIP.DB", "user")
		if err != nil {
			return nil, err
		}

		ipass, err = config.GetValue("VOIP.DB", "password")
		if err != nil {
			return nil, err
		}
	}
	chost, err := config.GetValue("CONTROLLER", "host")
	if err != nil {
		return nil, err
	}
	cport, err := config.GetValue("CONTROLLER", "port")
	if err != nil {
		return nil, err
	}

	hmap := make(map[string]string)
	for _, host := range hosts {
		address, err := config.GetValue("VOIP.TOPO", host)
		if err != nil {
			return nil, err
		}
		hmap[host] = address
	}

	return &DockerCManager{
		dockercls: make(map[string]*docker.DockerClient),
		hmap:      hmap,
		cadvisor: []string{"-storage_driver=influxdb",
			"-storage_driver_user=" + iuser,
			"-storage_driver_password=" + ipass,
			"-storage_driver_host=" + chost + ":" + cport,
			"-storage_driver_buffer_duration=" + BUF_DURATION},
		moncont: []string{chost + ":" + cport},
	}, nil
}

func (d *DockerCManager) Setup() error {
	undo := true
	err := ovsInit()
	if err != nil {
		return err
	}
	defer func() {
		if undo {
			ovsDestroy()
		}
	}()

	for host, address := range d.hmap {
		client, err := docker.NewDockerClient(address, nil)
		if err != nil {
			return err
		}

		d.dockercls[host] = client
		log.Println("[INFO] added host", host)

		id, err := client.CreateContainer(&docker.ContainerConfig{
			Image: IMG_CADVISOR,
			Cmd:   d.cadvisor,
		}, "cadvisor-"+host)
		if err != nil {
			return err
		}
		defer func(contid string, client *docker.DockerClient) {
			if undo {
				client.RemoveContainer(contid, true, true)
			}
		}(id, client)

		err = client.StartContainer(id, &docker.HostConfig{
			NetworkMode: "host",
			Binds: []string{"/:/rootfs:ro", "/var/run:/var/run:rw",
				"/sys:/sys:ro", "/var/lib/docker/:/var/lib/docker:ro"},
		})
		if err != nil {
			return err
		}
		defer func(contid string, client *docker.DockerClient) {
			if undo {
				client.StopContainer(contid, STOP_TIMEOUT)
			}
		}(id, client)
		log.Println("[INFO] running cadvisor on host", host)

		id, err = client.CreateContainer(&docker.ContainerConfig{
			Image: IMG_MONCONT,
			Cmd:   d.moncont,
		}, "moncont-"+host)
		if err != nil {
			return err
		}
		defer func(contid string, client *docker.DockerClient) {
			if undo {
				client.RemoveContainer(contid, true, true)
			}
		}(id, client)

		err = client.StartContainer(id, &docker.HostConfig{
			NetworkMode: "host",
			Binds: []string{"/proc:/host/proc:ro", "/var/run:/var/run:ro",
				"/var/lib/docker/:/var/lib/docker:ro"},
		})
		if err != nil {
			return err
		}
		defer func(contid string, client *docker.DockerClient) {
			if undo {
				client.StopContainer(contid, STOP_TIMEOUT)
			}
		}(id, client)
		log.Println("[INFO] running moncont on host", host)
	}

	undo = false
	return nil
}

func (d *DockerCManager) Destroy() {
	for host, client := range d.dockercls {
		contid := "cadvisor-" + host
		if err := client.StopContainer(contid, STOP_TIMEOUT); err != nil {
			log.Println("[WARN] unable to stop container", contid)
		} else {
			log.Println("[INFO] stopped cadvisor on host", host)
			client.RemoveContainer(contid, true, true)
		}

		contid = "moncont-" + host
		if err := client.StopContainer(contid, STOP_TIMEOUT); err != nil {
			log.Println("[WARN] unable to stop container", contid)
		} else {
			log.Println("[INFO] stopped moncont on host", host)
			client.RemoveContainer(contid, true, true)
		}
	}

	ovsDestroy()
}

func (d *DockerCManager) StartServer(host string, shares int64) (*Node, error) {
	return d.runc(host, "sipp-server", &docker.ContainerConfig{
		Env:             []string{"ARGS=-buff_size " + SIPP_BUFF_SIZE + " -sn uas"},
		Image:           IMG_SIPP,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares * CPU_PERIOD / 1024),
	})
}

func (d *DockerCManager) StartSnort(host string, shares int64) (*Node, error) {
	return d.runc(host, "snort", &docker.ContainerConfig{
		Image:           IMG_SNORT,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CapAdd:    []string{"NET_ADMIN"},
		CpuShares: shares,
		CpuQuota:  int64(shares * CPU_PERIOD / 1024),
	})
}

func (d *DockerCManager) StartClient(host string, shares int64, serverip string) (*Node, error) {
	args := "-buff_size " + SIPP_BUFF_SIZE + " -sn uac -r 0 " + serverip + ":5060"
	return d.runc(host, "sipp-client", &docker.ContainerConfig{
		Env:             []string{"ARGS=" + args},
		Image:           IMG_SIPP,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares * CPU_PERIOD / 1024),
	})
}

func (d *DockerCManager) StopCont(node *Node) error {
	client, ok := d.dockercls[node.host]
	if !ok {
		log.Println("[WARN] client doesn't exist for host", node.host)
		return ErrHostNotFound
	}

	ovsDeRoute(node.mac)
	log.Println("[INFO] derouted for container", node.id)

	err := client.StopContainer(node.id, STOP_TIMEOUT)
	if err != nil {
		log.Println("[WARN] unable to stop container", node.id)
	} else {
		log.Println("[INFO] container with id", node.id, "stopped")
		ovsUSetupNetwork(node.id)
		err = client.RemoveContainer(node.id, true, true)
	}

	return err
}

func (d *DockerCManager) Route(cnode, rnode, snode *Node) error {
	err := ovsRoute(cnode.mac, rnode.mac, snode.mac)
	if err != nil {
		return err
	}

	log.Println("[INFO] setup route", cnode.ip, "->", rnode.ip, "->", snode.ip)
	return nil
}

func (d *DockerCManager) SetShares(node *Node, shares int64) error {
	client, ok := d.dockercls[node.host]
	if !ok {
		log.Println("[WARN] docker client for host", node.host, "not found")
		return ErrHostNotFound
	}

	if err := client.SetContainer(node.id, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares * CPU_PERIOD / 1024),
	}); err != nil {
		log.Println("[WARN] unable to set new shares")
		return err
	}

	log.Println("[INFO] set cpu limit for container", node.id, "to", shares)
	return nil
}

func (d *DockerCManager) runc(host, prefix string, cconf *docker.ContainerConfig, hconf *docker.HostConfig) (*Node, error) {
	undo := true
	client, ok := d.dockercls[host]
	if !ok {
		return nil, ErrHostNotFound
	}

	cid := fmt.Sprintf("%s-%s", prefix, uuid.NewV1())
	_, err := client.CreateContainer(cconf, cid)
	if err != nil {
		return nil, err
	}
	defer func() {
		if undo {
			client.RemoveContainer(cid, true, true)
		}
	}()
	log.Println("[INFO] created container with id", cid)

	err = client.StartContainer(cid, hconf)
	if err != nil {
		return nil, err
	}
	defer func() {
		if undo {
			client.StopContainer(cid, STOP_TIMEOUT)
		}
	}()
	log.Println("[INFO] started container with id", cid)

	ip, mac, err := ovsSetupNetwork(cid)
	if err != nil {
		return nil, err
	}
	defer func() {
		if undo {
			ovsUSetupNetwork(cid)
		}
	}()
	log.Println("[INFO] setup network for container", cid, "ip:", ip, "mac:", mac)

	undo = false
	return NewNode(cid, ip, mac, host), nil
}
