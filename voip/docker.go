package voip

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Unknwon/goconfig"
	"github.com/samalba/dockerclient"
	"github.com/satori/go.uuid"
)

const (
	IMG_SIPP       = "mangalaman93/sipp"
	IMG_SNORT      = "mangalaman93/snort"
	SIPP_BUFF_SIZE = "1048576"
	CPU_PERIOD     = 100000
	STOP_TIMEOUT   = 4
)

var (
	ErrHostNotFound = errors.New("Host not found")
	ErrNoMacAddress = errors.New("Unable to generate mac address")
)

type DockerCManager struct {
	dclients map[string]*dockerclient.DockerClient
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

	dclients := make(map[string]*dockerclient.DockerClient)
	for _, host := range hosts {
		address, err := config.GetValue("VOIP.TOPO", host)
		if err != nil {
			return nil, err
		}

		c, err := dockerclient.NewDockerClient(address, nil)
		if err != nil {
			return nil, err
		}

		dclients[host] = c
	}

	return &DockerCManager{
		dclients: dclients,
		pipe:     NewPipeLine(),
	}, nil
}

func (dc *DockerCManager) Destroy() {
	ovsDestroy()
}

func (dc *DockerCManager) AddServer(cmd *Command) *Response {
	undo := false

	// get all the required keys from KeyVal first
	kv := cmd.KeyVal
	host, ok := kv["host"]
	if !ok {
		return &Response{err: ErrKeyNotFound}
	}
	hclient, ok := dc.dclients[host]
	if !ok {
		return &Response{err: ErrHostNotFound}
	}

	sshares, ok := kv["shares"]
	if !ok {
		return &Response{err: ErrKeyNotFound}
	}
	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{err: err}
	}

	mac, err := GetMacAddress()
	if err != nil {
		return &Response{err: ErrNoMacAddress}
	}

	cid, err := hclient.CreateContainer(&dockerclient.ContainerConfig{
		Env:             []string{"ARGS=\"-buff_size " + SIPP_BUFF_SIZE + " -sn uas\""},
		Image:           IMG_SIPP,
		NetworkDisabled: true,
	}, fmt.Sprintf("sipp-server-%s", uuid.NewV1()))
	if err != nil {
		return &Response{err: err}
	}
	defer func() {
		if undo {
			hclient.RemoveContainer(cid, true, true)
		}
	}()

	// setup ovs network
	err = ovsSetupNetwork(cid, mac)
	if err != nil {
		undo = true
		return &Response{err: err}
	}

	// add node to the pipe
	info, err := hclient.InspectContainer(cid)
	err = dc.pipe.NewNode(cid, info.NetworkSettings.IPAddress, mac, host)
	if err != nil {
		undo = true
		return &Response{err: err}
	}

	err = hclient.StartContainer(cid, &dockerclient.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares / 1024 * CPU_PERIOD),
	})
	if err != nil {
		undo = true
		return &Response{err: err}
	}

	return &Response{result: cid}
}

func (dc *DockerCManager) AddClient(cmd *Command) *Response {
	undo := false

	// get all the required keys from KeyVal first
	kv := cmd.KeyVal
	host, ok := kv["host"]
	if !ok {
		return &Response{err: ErrKeyNotFound}
	}
	hclient, ok := dc.dclients[host]
	if !ok {
		return &Response{err: ErrHostNotFound}
	}

	sshares, ok := kv["shares"]
	if !ok {
		return &Response{err: ErrKeyNotFound}
	}
	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{err: err}
	}

	serverid, ok := kv["server"]
	if !ok {
		return &Response{err: ErrKeyNotFound}
	}

	server_ip, err := dc.pipe.GetIPAddress(serverid)
	if err != nil {
		return &Response{err: err}
	}

	mac, err := GetMacAddress()
	if err != nil {
		return &Response{err: ErrNoMacAddress}
	}

	args := "-buff_size " + SIPP_BUFF_SIZE + " -sn uac -r 0 " + server_ip + ":5060"
	sid, err := hclient.CreateContainer(&dockerclient.ContainerConfig{
		Env:             []string{"ARGS=\"" + args + "\""},
		Image:           IMG_SIPP,
		NetworkDisabled: true,
	}, fmt.Sprintf("sipp-client-%s", uuid.NewV1()))
	if err != nil {
		return &Response{err: err}
	}
	defer func() {
		if undo {
			hclient.RemoveContainer(sid, true, true)
		}
	}()

	// setup ovs network
	err = ovsSetupNetwork(sid, mac)
	if err != nil {
		undo = true
		return &Response{err: err}
	}

	// add node to the pipe
	info, err := hclient.InspectContainer(sid)
	err = dc.pipe.NewNode(sid, info.NetworkSettings.IPAddress, mac, host)
	if err != nil {
		undo = true
		return &Response{err: err}
	}

	err = hclient.StartContainer(sid, &dockerclient.HostConfig{
		CpuShares: shares,
		CpuQuota:  int64(shares / 1024 * CPU_PERIOD),
	})
	if err != nil {
		undo = true
		return &Response{err: err}
	}

	return &Response{result: sid}
}

func (dc *DockerCManager) AddSnort(cmd *Command) *Response {
	undo := false

	// get all the required keys from KeyVal first
	kv := cmd.KeyVal
	host, ok := kv["host"]
	if !ok {
		return &Response{err: ErrKeyNotFound}
	}
	hclient, ok := dc.dclients[host]
	if !ok {
		return &Response{err: ErrHostNotFound}
	}

	sshares, ok := kv["shares"]
	if !ok {
		return &Response{err: ErrKeyNotFound}
	}
	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{err: err}
	}

	mac, err := GetMacAddress()
	if err != nil {
		return &Response{err: ErrNoMacAddress}
	}

	id, err := hclient.CreateContainer(&dockerclient.ContainerConfig{
		Image:           IMG_SNORT,
		NetworkDisabled: true,
	}, fmt.Sprintf("snort-%s", uuid.NewV1()))
	if err != nil {
		return &Response{err: err}
	}
	defer func() {
		if undo {
			hclient.RemoveContainer(id, true, true)
		}
	}()

	// setup ovs network
	err = ovsSetupNetwork(id, mac)
	if err != nil {
		undo = true
		return &Response{err: err}
	}

	// add node to the pipe
	info, err := hclient.InspectContainer(id)
	err = dc.pipe.NewNode(id, info.NetworkSettings.IPAddress, mac, host)
	if err != nil {
		undo = true
		return &Response{err: err}
	}

	err = hclient.StartContainer(id, &dockerclient.HostConfig{
		CapAdd:    []string{"NET_ADMIN"},
		CpuShares: shares,
		CpuQuota:  int64(shares / 1024 * CPU_PERIOD),
	})
	if err != nil {
		undo = true
		return &Response{err: err}
	}

	return &Response{result: id}
}

func (dc *DockerCManager) Stop(cmd *Command) *Response {
	// get all the required keys from KeyVal first
	kv := cmd.KeyVal
	cont, ok := kv["cont"]
	if !ok {
		return &Response{err: ErrKeyNotFound}
	}
	host, err := dc.pipe.GetHost(cont)
	if err != nil {
		return &Response{err: err}
	}
	dclient, ok := dc.dclients[host]
	if !ok {
		// Impossible
		panic(!ok)
	}

	err = dclient.StopContainer(cont, STOP_TIMEOUT)
	if err != nil {
		return &Response{err: err}
	}

	err = dclient.RemoveContainer(cont, true, true)
	if err != nil {
		return &Response{err: err}
	}

	err = dc.pipe.DelNode(cont)
	if err != nil {
		return &Response{err: err}
	}

	return &Response{}
}

func (dc *DockerCManager) Route(cmd *Command) *Response {
	return nil
}
