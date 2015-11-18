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
	img_sipp       = "mangalaman93/sipp"
	img_snort      = "mangalaman93/snort"
	img_cadvisor   = "mangalaman93/cadvisor"
	sipp_buff_size = "1048576"
	cpu_period     = 100000
	stop_timeout   = 4
)

var (
	ErrHostNotFound = errors.New("Host not found")
	ErrNoMacAddress = errors.New("Unable to generate mac address")
)

type DockerCManager struct {
	dclients map[string]*docker.DockerClient
	pipe     *PipeLine
}

func NewDockerCManager(config *goconfig.ConfigFile) (*DockerCManager, error) {
	undo := true

	hosts := config.GetKeyList("VOIP.TOPO")
	if hosts == nil {
		return nil, errors.New("error while finding host list")
	}

	iuser, err := config.GetValue("VOIP.DB", "user")
	if err != nil {
		return nil, err
	}

	ipass, err := config.GetValue("VOIP.DB", "password")
	if err != nil {
		return nil, err
	}

	ihost, err := config.GetValue("VOIP.DB", "host")
	if err != nil {
		return nil, err
	}

	iport, err := config.GetValue("VOIP.DB", "port")
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

	dclients := make(map[string]*docker.DockerClient)
	for _, host := range hosts {
		address, err := config.GetValue("VOIP.TOPO", host)
		if err != nil {
			return nil, err
		}

		c, err := docker.NewDockerClient(address, nil)
		if err != nil {
			return nil, err
		}

		dclients[host] = c
		log.Println("[_INFO] added host", host)

		id, err := c.CreateContainer(&docker.ContainerConfig{
			Image: img_cadvisor,
			Cmd: []string{"-storage_driver=influxdb",
				"-storage_driver_user=" + iuser,
				"-storage_driver_password=" + ipass,
				"-storage_driver_host=" + ihost + ":" + iport},
		}, "cadvisor-"+host)
		if err != nil {
			return nil, err
		}
		defer func(contid string, client *docker.DockerClient) {
			if undo {
				client.RemoveContainer(contid, true, true)
			}
		}(id, c)

		err = c.StartContainer(id, &docker.HostConfig{
			NetworkMode: "host",
			Binds:       []string{"/:/rootfs:ro", "/var/run:/var/run:rw", "/sys:/sys:ro", "/var/lib/docker/:/var/lib/docker:ro"},
		})
		if err != nil {
			return nil, err
		}
		defer func(contid string, client *docker.DockerClient) {
			if undo {
				client.StopContainer(contid, stop_timeout)
			}
		}(id, c)

		log.Println("[_INFO] running cadvisor on host", host)
	}

	undo = false
	return &DockerCManager{
		dclients: dclients,
		pipe:     NewPipeLine(),
	}, nil
}

func (dc *DockerCManager) Destroy() {
	dc.pipe.Destroy(dc)
	ovsDestroy()

	for host, client := range dc.dclients {
		contid := "cadvisor-" + host

		if err := client.StopContainer(contid, stop_timeout); err != nil {
			log.Println("[_WARN] unable to stop container", contid)
		} else {
			log.Println("[_INFO] container with id", contid, "stopped")
			client.RemoveContainer(contid, true, true)
		}

		log.Println("[_INFO] stopped cadvisor on host", host)
	}
}

func (dc *DockerCManager) AddServer(cmd *Command) *Response {
	kv := cmd.KeyVal
	host, ok1 := kv["host"]
	sshares, ok2 := kv["shares"]
	if !ok1 || !ok2 {
		return &Response{Err: ErrKeyNotFound.Error()}
	}

	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	return dc.runc(host, "sipp-server", &docker.ContainerConfig{
		Env:             []string{"ARGS=\"-buff_size " + sipp_buff_size + " -sn uas\""},
		Image:           img_sipp,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares / 1024 * cpu_period),
	})
}

func (dc *DockerCManager) AddClient(cmd *Command) *Response {
	kv := cmd.KeyVal
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

	server_ip, err := dc.pipe.GetIPAddress(serverid)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	args := "-buff_size " + sipp_buff_size + " -sn uac -r 0 " + server_ip + ":5060"
	return dc.runc(host, "sipp-client", &docker.ContainerConfig{
		Env:             []string{"ARGS=\"" + args + "\""},
		Image:           img_sipp,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares / 1024 * cpu_period),
	})
}

func (dc *DockerCManager) AddSnort(cmd *Command) (*Response, string) {
	kv := cmd.KeyVal
	host, ok1 := kv["host"]
	sshares, ok2 := kv["shares"]
	if !ok1 || !ok2 {
		return &Response{Err: ErrKeyNotFound.Error()}, ""
	}

	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{Err: err.Error()}, ""
	}

	return dc.runc(host, "snort", &docker.ContainerConfig{
		Image:           img_snort,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CapAdd:    []string{"NET_ADMIN"},
		CpuShares: shares,
		CpuQuota:  int64(shares / 1024 * cpu_period),
	}), sshares
}

func (dc *DockerCManager) runc(host, prefix string, cconf *docker.ContainerConfig, hconf *docker.HostConfig) *Response {
	hclient, ok := dc.dclients[host]
	if !ok {
		return &Response{Err: ErrHostNotFound.Error()}
	}

	// rollback in case!
	undo := true

	cid, err := hclient.CreateContainer(cconf, fmt.Sprintf("%s-%s", prefix, uuid.NewV1()))
	if err != nil {
		return &Response{Err: err.Error()}
	}
	defer func() {
		if undo {
			hclient.RemoveContainer(cid, true, true)
		}
	}()
	log.Println("[_INFO] created container with id", cid)

	err = hclient.StartContainer(cid, hconf)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	log.Println("[_INFO] container with id", cid)
	defer func() {
		if undo {
			hclient.StopContainer(cid, stop_timeout)
		}
	}()

	// setup ovs network
	ip, mac, err := ovsSetupNetwork(cid)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	log.Println("[_INFO] setup network for container", cid, "ip:", ip, "mac:", mac)
	defer func() {
		if undo {
			ovsUSetupNetwork(cid)
		}
	}()

	err = dc.pipe.NewNode(cid, ip, mac, host)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	defer func() {
		if undo {
			dc.pipe.DelNode(cid)
		}
	}()

	undo = false
	return &Response{Result: cid}
}

func (dc *DockerCManager) Stop(cmd *Command) *Response {
	kv := cmd.KeyVal
	cont, ok := kv["cont"]
	if !ok {
		return &Response{Err: ErrKeyNotFound.Error()}
	}

	host, err := dc.pipe.GetHost(cont)
	if err != nil {
		return &Response{Err: err.Error()}
	}
	hclient, ok := dc.dclients[host]
	if !ok {
		panic(!ok)
	}

	err = hclient.StopContainer(cont, stop_timeout)
	if err != nil {
		log.Println("[_WARN] unable to stop container", cont)
	} else {
		log.Println("[_INFO] container with id", cont, "stopped")
		ovsUSetupNetwork(cont)
		hclient.RemoveContainer(cont, true, true)
	}

	cmac, err := dc.pipe.GetMacAddress(cont)
	if err != nil {
		log.Println("[_WARN] unable to get mac from pipe", cont)
		log.Println("[_WARN] pipe may be inconsistent")
	} else {
		ovsDeRoute(cmac)
		log.Println("[_INFO] derouted for container", cont)
	}

	err = dc.pipe.DelNode(cont)
	if err != nil {
		log.Println("[_WARN] unable to delete node from pipe", cont, "err:", err)
		log.Println("[_WARN] pipe may be inconsistent")
	}

	return &Response{}
}

func (dc *DockerCManager) Route(cmd *Command) *Response {
	// get all the required keys from KeyVal first
	kv := cmd.KeyVal
	client, ok1 := kv["client"]
	server, ok2 := kv["server"]
	router, ok3 := kv["router"]
	if !ok1 || !ok2 || !ok3 {
		return &Response{Err: ErrKeyNotFound.Error()}
	}

	cmac, err := dc.pipe.GetMacAddress(client)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	smac, err := dc.pipe.GetMacAddress(server)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	rmac, err := dc.pipe.GetMacAddress(router)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	err = ovsRoute(cmac, rmac, smac)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	err = dc.pipe.AddNode(server, RootNode.id)
	if err != nil {
		panic(err)
	}

	err = dc.pipe.AddNode(router, server)
	if err != nil {
		panic(err)
	}

	err = dc.pipe.AddNode(client, router)
	if err != nil {
		panic(err)
	}

	log.Println("[_INFO] setup route", cmac, "->", rmac, "->", smac)
	return &Response{}
}

func (dc *DockerCManager) SetShares(id string, shares int64) {
	host, err := dc.pipe.GetHost(id)
	if err != nil {
		log.Println("[_WARN] unable to set shares for container", id, "err:", err)
		return
	}

	hclient, ok := dc.dclients[host]
	if !ok {
		panic(!ok)
	}

	if err := hclient.SetContainer(id, &docker.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares / 1024 * cpu_period),
	}); err != nil {
		log.Println("[_WARN] unable to set new shares")
	}

	log.Println("[_INFO] set cpu limit for container", id, "to", shares)
}
