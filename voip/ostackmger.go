package voip

import (
	"fmt"
	"log"
	"strings"

	"github.com/Unknwon/goconfig"
	docker "github.com/mangalaman93/dockerclient"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud/openstack/compute/v2/servers"
)

const (
	WAIT_FOR_START = 10
)

type OStackCManager struct {
	osclient  *gophercloud.ServiceClient
	dockercls map[string]*docker.DockerClient
	hmap      map[string]string
	cadvisor  []string
	moncont   []string
}

func NewOStackCManager(config *goconfig.ConfigFile) (*OStackCManager, error) {
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
	database, err := config.GetValue("VOIP", "db")
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

	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, err
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, err
	}
	osclient, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{Region: "RegionOne"})
	if err != nil {
		return nil, err
	}

	return &OStackCManager{
		osclient:  osclient,
		dockercls: make(map[string]*docker.DockerClient),
		hmap:      hmap,
		cadvisor: []string{"-storage_driver=influxdb",
			"-storage_driver_user=" + iuser,
			"-storage_driver_password=" + ipass,
			"-storage_driver_host=" + chost + ":" + cport,
			"-storage_driver_db=" + database,
			"-storage_driver_buffer_duration=" + BUF_DURATION},
		moncont: []string{chost + ":" + cport},
	}, nil
}

func (o *OStackCManager) Setup() error {
	undo := true

	for host, address := range o.hmap {
		tokens := strings.Split(address, ":")
		if len(tokens) < 2 {
			return fmt.Errorf("Unexpected host address")
		}
		client, err := docker.NewDockerClient(address, nil)
		if err != nil {
			return err
		}
		o.hmap[host] = tokens[0]

		o.dockercls[host] = client
		log.Println("[INFO] added host", host)

		id, err := client.CreateContainer(&docker.ContainerConfig{
			Image: IMG_CADVISOR,
			Cmd:   o.cadvisor,
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
			Cmd:   o.moncont,
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

func (o *OStackCManager) Destroy() {
	for host, client := range o.dockercls {
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
}

func (o *OStackCManager) StartServer(host string, shares int64) (*Node, error) {
	return o.runc(host, "sipp-server", shares, servers.CreateOpts{
		Name:             "sipp-server",
		FlavorName:       "c1.tiny",
		ImageName:        IMG_SIPP,
		Metadata:         map[string]string{"ARGS": "-buff_size " + SIPP_BUFF_SIZE + " -sn uas"},
		AvailabilityZone: "regionOne:" + host,
	})
}

func (o *OStackCManager) StartSnort(host string, shares int64) (*Node, error) {
	return o.runc(host, "snort", shares, servers.CreateOpts{
		Name:             "snort",
		FlavorName:       "c1.tiny",
		ImageName:        IMG_SNORT,
		Metadata:         map[string]string{"OPT_CAP_ADD": "NET_ADMIN"},
		AvailabilityZone: "regionOne:" + host,
	})
}

func (o *OStackCManager) StartClient(host string, shares int64, serverip string) (*Node, error) {
	args := "-buff_size " + SIPP_BUFF_SIZE + " -sn uac -r 0 " + serverip + ":5060"
	return o.runc(host, "sipp-client", shares, servers.CreateOpts{
		Name:             "sipp-client",
		FlavorName:       "c1.tiny",
		ImageName:        IMG_SIPP,
		Metadata:         map[string]string{"ARGS": args},
		AvailabilityZone: "regionOne:" + host,
	})
}

func (o *OStackCManager) StopCont(node *Node) error {
	address, ok := o.hmap[node.host]
	if !ok {
		log.Println("[WARN] address for host:", node.host, "not found")
		return ErrHostNotFound
	}
	ovsosDeRoute(address, node.mac)
	log.Println("[INFO] derouted for container", node.id)

	err := servers.Delete(o.osclient, node.id).ExtractErr()
	if err != nil {
		log.Println("[WARN] unable to stop container", node.id)
	} else {
		log.Println("[INFO] container with id", node.id, "stopped")
	}
	return err
}

func (o *OStackCManager) Route(cnode, rnode, snode *Node) error {
	address, ok := o.hmap[cnode.host]
	if !ok {
		log.Println("[WARN] address for host:", cnode.host, "not found")
		return ErrHostNotFound
	}
	err := ovsosRoute(address, cnode.mac, rnode.mac, snode.mac)
	if err != nil {
		return err
	}

	log.Println("[INFO] setup route", cnode.ip, "->", rnode.ip, "->", snode.ip)
	return nil
}

func (o *OStackCManager) SetShares(node *Node, shares int64) error {
	client, ok := o.dockercls[node.host]
	if !ok {
		log.Println("[WARN] docker client for host", node.host, "not found")
		return ErrHostNotFound
	}

	if err := client.SetContainer(node.other, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares * CPU_PERIOD / 1024),
	}); err != nil {
		log.Println("[WARN] unable to set new shares")
		return err
	}

	log.Println("[INFO] set cpu limit for container", node.other, "to", shares)
	return nil
}

func (o *OStackCManager) runc(host, prefix string, shares int64, opts servers.CreateOpts) (*Node, error) {
	undo := true
	cont, err := servers.Create(o.osclient, opts).Extract()
	if err != nil {
		return nil, err
	}
	defer func() {
		if undo {
			servers.Delete(o.osclient, cont.ID)
		}
	}()
	err = servers.WaitForStatus(o.osclient, cont.ID, "ACTIVE", WAIT_FOR_START)
	if err != nil {
		return nil, err
	}
	log.Println("[INFO] started container with id", cont.ID)

	// TODO (temporary solution)
	out, err := runsh("nova --os-username admin --os-tenant-name admin --os-password ubuntu --os-auth-url http://10.1.21.14:5000/v2.0 interface-list " + cont.ID)
	tokens := strings.Split(string(out), "|")
	if len(tokens) < 12 {
		return nil, fmt.Errorf("Mac address not found!")
	}

	ip := strings.TrimSpace(strings.Split(tokens[10], ",")[0])
	node := NewNode(cont.ID, ip, strings.TrimSpace(tokens[11]), host)
	node.other = prefix + "-" + cont.ID
	err = o.SetShares(node, shares)
	if err != nil {
		return nil, err
	}

	undo = false
	return node, nil
}
