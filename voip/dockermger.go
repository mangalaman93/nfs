package voip

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/Unknwon/goconfig"
	docker "github.com/mangalaman93/dockerclient"
	"github.com/satori/go.uuid"
)

const (
	IMG_SIPP              = "mangalaman93/sipp"
	IMG_SNORT             = "mangalaman93/snort"
	IMG_CADVISOR          = "mangalaman93/cadvisor"
	SIPP_BUFF_SIZE        = "1048576"
	DEFAULT_CPU_PERIOD    = 100000
	CONT_STOP_TIMEOUT     = 5
	CADVISOR_BUF_DURATION = "5s"
)

var (
	ErrHostNotFound = errors.New("Host not found")
	ErrNoMacAddress = errors.New("Unable to generate mac address")
)

type DockerCManager struct {
	dokclients map[string]*docker.DockerClient
	pipe       *PipeLine
}

func NewDockerCManager(config *goconfig.ConfigFile) (*DockerCManager, error) {
	undo := true

	hosts := config.GetKeyList("VOIP.TOPO")
	if hosts == nil {
		return nil, errors.New("error while finding host list")
	}

	var iuser, ipass string
	var err error
	if s, _ := config.GetSection("VOIP.DB"); s == nil {
		iuser = ""
		ipass = ""
	} else {
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

	err = ovsInit()
	if err != nil {
		return nil, err
	}
	defer func() {
		if undo {
			ovsDestroy()
		}
	}()

	dokclients := make(map[string]*docker.DockerClient)
	for _, host := range hosts {
		address, err := config.GetValue("VOIP.TOPO", host)
		if err != nil {
			return nil, err
		}
		client, err := docker.NewDockerClient(address, nil)
		if err != nil {
			return nil, err
		}
		dokclients[host] = client
		log.Println("[INFO] added host", host)

		id, err := client.CreateContainer(&docker.ContainerConfig{
			Image: IMG_CADVISOR,
			Cmd: []string{"-storage_driver=influxdb",
				"-storage_driver_user=" + iuser,
				"-storage_driver_password=" + ipass,
				"-storage_driver_host=" + chost + ":" + cport,
				"-storage_driver_buffer_duration=" + CADVISOR_BUF_DURATION},
		}, "cadvisor-"+host)
		if err != nil {
			return nil, err
		}
		defer func(contid string, client *docker.DockerClient) {
			if undo {
				client.RemoveContainer(contid, true, true)
			}
		}(id, client)

		err = client.StartContainer(id, &docker.HostConfig{
			NetworkMode: "host",
			Binds:       []string{"/:/rootfs:ro", "/var/run:/var/run:rw", "/sys:/sys:ro", "/var/lib/docker/:/var/lib/docker:ro"},
		})
		if err != nil {
			return nil, err
		}
		defer func(contid string, client *docker.DockerClient) {
			if undo {
				client.StopContainer(contid, CONT_STOP_TIMEOUT)
			}
		}(id, client)
		log.Println("[INFO] running cadvisor on host", host)
	}

	undo = false
	return &DockerCManager{
		dokclients: dokclients,
		pipe:       NewPipeLine(),
	}, nil
}

func (d *DockerCManager) Destroy() {
	d.pipe.Destroy(d)
	ovsDestroy()

	for host, client := range d.dokclients {
		contid := "cadvisor-" + host
		if err := client.StopContainer(contid, CONT_STOP_TIMEOUT); err != nil {
			log.Println("[WARN] unable to stop container", contid)
		} else {
			log.Println("[INFO] container with id", contid, "stopped")
			client.RemoveContainer(contid, true, true)
		}

		log.Println("[INFO] stopped cadvisor on host", host)
	}
}

func (d *DockerCManager) AddServer(req *Request) *Response {
	kv := req.KeyVal
	host, ok1 := kv["host"]
	sshares, ok2 := kv["shares"]
	if !ok1 || !ok2 {
		return &Response{Err: ErrKeyNotFound.Error()}
	}
	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	return d.runc(host, "sipp-server", &docker.ContainerConfig{
		Env:             []string{"ARGS=-buff_size " + SIPP_BUFF_SIZE + " -sn uas"},
		Image:           IMG_SIPP,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares * DEFAULT_CPU_PERIOD / 1024),
	})
}

func (d *DockerCManager) AddClient(req *Request) *Response {
	kv := req.KeyVal
	host, ok1 := kv["host"]
	sshares, ok2 := kv["shares"]
	serverid, ok3 := kv["server"]
	if !ok1 || !ok2 || !ok3 {
		return &Response{Err: ErrKeyNotFound.Error()}
	}
	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	serverip, err := d.pipe.GetIPAddress(serverid)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	args := "-buff_size " + SIPP_BUFF_SIZE + " -sn uac -r 0 " + serverip + ":5060"
	return d.runc(host, "sipp-client", &docker.ContainerConfig{
		Env:             []string{"ARGS=" + args},
		Image:           IMG_SIPP,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares * DEFAULT_CPU_PERIOD / 1024),
	})
}

func (d *DockerCManager) AddSnort(req *Request) (*Response, string) {
	kv := req.KeyVal
	host, ok1 := kv["host"]
	sshares, ok2 := kv["shares"]
	if !ok1 || !ok2 {
		return &Response{Err: ErrKeyNotFound.Error()}, ""
	}
	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{Err: err.Error()}, ""
	}

	return d.runc(host, "snort", &docker.ContainerConfig{
		Image:           IMG_SNORT,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CapAdd:    []string{"NET_ADMIN"},
		CpuShares: shares,
		CpuQuota:  int64(shares * DEFAULT_CPU_PERIOD / 1024),
	}), sshares
}

func (d *DockerCManager) runc(host, prefix string, cconf *docker.ContainerConfig, hconf *docker.HostConfig) *Response {
	undo := true

	client, ok := d.dokclients[host]
	if !ok {
		return &Response{Err: ErrHostNotFound.Error()}
	}

	cid := fmt.Sprintf("%s-%s", prefix, uuid.NewV1())
	_, err := client.CreateContainer(cconf, cid)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	defer func() {
		if undo {
			client.RemoveContainer(cid, true, true)
		}
	}()
	log.Println("[INFO] created container with id", cid)

	err = client.StartContainer(cid, hconf)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	log.Println("[INFO] container with id", cid)
	defer func() {
		if undo {
			client.StopContainer(cid, CONT_STOP_TIMEOUT)
		}
	}()

	ip, mac, err := ovsSetupNetwork(cid)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	log.Println("[INFO] setup network for container", cid, "ip:", ip, "mac:", mac)
	defer func() {
		if undo {
			ovsUSetupNetwork(cid)
		}
	}()

	err = d.pipe.NewNode(cid, ip, mac, host)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	defer func() {
		if undo {
			d.pipe.DelNode(cid)
		}
	}()

	undo = false
	return &Response{Result: cid}
}

func (d *DockerCManager) Stop(req *Request) *Response {
	kv := req.KeyVal
	cont, ok := kv["cont"]
	if !ok {
		return &Response{Err: ErrKeyNotFound.Error()}
	}
	host, err := d.pipe.GetHost(cont)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	client, ok := d.dokclients[host]
	if !ok {
		panic(!ok)
	}

	err = client.StopContainer(cont, CONT_STOP_TIMEOUT)
	if err != nil {
		log.Println("[WARN] unable to stop container", cont)
	} else {
		log.Println("[INFO] container with id", cont, "stopped")
		ovsUSetupNetwork(cont)
		client.RemoveContainer(cont, true, true)
	}

	cmac, err := d.pipe.GetMacAddress(cont)
	if err != nil {
		log.Println("[WARN] unable to get mac from pipe", cont)
		log.Println("[WARN] pipe may be inconsistent")
	} else {
		ovsDeRoute(cmac)
		log.Println("[INFO] derouted for container", cont)
	}

	err = d.pipe.DelNode(cont)
	if err != nil {
		log.Println("[WARN] unable to delete node from pipe", cont, "err:", err)
		log.Println("[WARN] pipe may be inconsistent")
	}

	return &Response{}
}

func (d *DockerCManager) Route(req *Request) *Response {
	// get all the required keys from KeyVal first
	kv := req.KeyVal
	client, ok1 := kv["client"]
	server, ok2 := kv["server"]
	router, ok3 := kv["router"]
	if !ok1 || !ok2 || !ok3 {
		return &Response{Err: ErrKeyNotFound.Error()}
	}

	cmac, err := d.pipe.GetMacAddress(client)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	smac, err := d.pipe.GetMacAddress(server)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	rmac, err := d.pipe.GetMacAddress(router)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	err = ovsRoute(cmac, rmac, smac)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	err = d.pipe.AddNode(server, RootNode.id)
	if err != nil {
		panic(err)
	}
	err = d.pipe.AddNode(router, server)
	if err != nil {
		panic(err)
	}
	err = d.pipe.AddNode(client, router)
	if err != nil {
		panic(err)
	}

	log.Println("[INFO] setup route", cmac, "->", rmac, "->", smac)
	return &Response{}
}

func (d *DockerCManager) SetShares(id string, shares int64) {
	host, err := d.pipe.GetHost(id)
	if err != nil {
		log.Println("[WARN] unable to set shares for container", id, "err:", err)
		return
	}
	client, ok := d.dokclients[host]
	if !ok {
		panic(!ok)
	}

	if err := client.SetContainer(id, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares * DEFAULT_CPU_PERIOD / 1024),
	}); err != nil {
		log.Println("[WARN] unable to set new shares")
	}

	log.Println("[INFO] set cpu limit for container", id, "to", shares)
}
