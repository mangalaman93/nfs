package voip

import (
	"errors"

	"github.com/Unknwon/goconfig"
	"github.com/samalba/dockerclient"
)

const (
	IMG_SIPP_SERVER = "mangalaman93/sipp"
)

type DockerCManager struct {
	dclient map[string]*dockerclient.DockerClient
}

func NewDockerCManager(config *goconfig.ConfigFile) (*DockerCManager, error) {
	hosts := config.GetKeyList("VOIP.TOPO")
	if hosts == nil {
		return nil, errors.New("error while finding host list")
	}

	dclient := make(map[string]*dockerclient.DockerClient)
	for _, host := range hosts {
		address, err := config.GetValue("VOIP.TOPO", host)
		if err != nil {
			return nil, err
		}

		c, err := dockerclient.NewDockerClient(address, nil)
		if err != nil {
			return nil, err
		}

		dclient[host] = c
	}

	return &DockerCManager{
		dclient: dclient,
	}, nil
}

func (dc *DockerCManager) AddServer(cmd *Command) *Response {
	return nil
}

func (dc *DockerCManager) AddClient(cmd *Command) *Response {
	return nil
}

func (dc *DockerCManager) AddSnort(cmd *Command) *Response {
	return nil
}

func (dc *DockerCManager) Stop(cmd *Command) *Response {
	return nil
}

func (dc *DockerCManager) Route(cmd *Command) *Response {
	return nil
}
