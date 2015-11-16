package voip

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/Unknwon/goconfig"
	docker "github.com/samalba/dockerclient"
	"github.com/satori/go.uuid"
)

const (
	img_sipp       = "mangalaman93/sipp"
	img_snort      = "mangalaman93/snort"
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
	hosts := config.GetKeyList("VOIP.TOPO")
	if hosts == nil {
		return nil, errors.New("error while finding host list")
	}

	err := ovsInit()
	if err != nil {
		return nil, err
	}

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
	}

	return &DockerCManager{
		dclients: dclients,
		pipe:     NewPipeLine(),
	}, nil
}

func (dc *DockerCManager) Destroy() {
	dc.pipe.Destroy(dc)
	ovsDestroy()
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

func (dc *DockerCManager) AddSnort(cmd *Command) *Response {
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

	return dc.runc(host, "snort", &docker.ContainerConfig{
		Image:           img_snort,
		NetworkDisabled: true,
	}, &docker.HostConfig{
		CapAdd:    []string{"NET_ADMIN"},
		CpuShares: shares,
		CpuQuota:  int64(shares / 1024 * cpu_period),
	})
}

func (dc *DockerCManager) runc(host, prefix string, cconf *docker.ContainerConfig, hconf *docker.HostConfig) *Response {
	hclient, ok := dc.dclients[host]
	if !ok {
		return &Response{Err: ErrHostNotFound.Error()}
	}

	// rollback in case!
	undo := false

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

	// setup ovs network
	ip, mac, err := ovsSetupNetwork(cid)
	if err != nil {
		undo = true
		return &Response{Err: err.Error()}
	}
	log.Println("[_INFO] setup network for container", cid)
	defer func() {
		if undo {
			ovsUSetupNetwork(cid)
		}
	}()

	err = dc.pipe.NewNode(cid, ip, mac, host)
	if err != nil {
		undo = true
		return &Response{Err: err.Error()}
	}
	defer func() {
		if undo {
			dc.pipe.DelNode(cid)
		}
	}()

	err = hclient.StartContainer(cid, hconf)
	if err != nil {
		undo = true
		return &Response{Err: err.Error()}
	}
	log.Println("[_INFO] container with id", cid, "running with ip", ip)

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

	// TODO: ideally, the call should be explicit
	if strings.Contains(cont, "sipp-client") {
		cmac, err := dc.pipe.GetMacAddress(cont)
		if err != nil {
			log.Println("[_WARN] unable to get mac from pipe", cont)
			log.Println("[_WARN] pipe may be inconsistent")
		} else {
			ovsDeRoute(cmac)
			log.Println("[_INFO] derouted for container", cont)
		}
	}

	err = dc.pipe.DelNode(cont)
	if err != nil {
		log.Println("[_WARN] unable to delete node from pipe", cont)
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

	err = dc.pipe.AddNode(client, "")
	if err != nil {
		panic(err)
	}

	err = dc.pipe.AddNode(router, client)
	if err != nil {
		panic(err)
	}

	err = dc.pipe.AddNode(server, router)
	if err != nil {
		panic(err)
	}

	log.Println("[_INFO] setup route", cmac, " -> ", rmac, " -> ", smac)
	return &Response{}
}
