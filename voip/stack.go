package voip

import (
	"github.com/Unknwon/goconfig"
	"github.com/rackspace/gophercloud"
)

type StackCManager struct {
	sclient map[string]*gophercloud.ProviderClient
}

func NewStackCManager(config *goconfig.ConfigFile) (*StackCManager, error) {
	return nil, nil
}

func (s *StackCManager) AddServer(cmd *Command) *Response {
	return nil
}

func (dc *StackCManager) AddClient(cmd *Command) *Response {
	return nil
}

func (dc *StackCManager) AddSnort(cmd *Command) *Response {
	return nil
}

func (dc *StackCManager) Stop(cmd *Command) *Response {
	return nil
}

func (dc *StackCManager) Route(cmd *Command) *Response {
	return nil
}
