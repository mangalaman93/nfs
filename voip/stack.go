package voip

import (
	_ "os"

	"github.com/Unknwon/goconfig"
	"github.com/rackspace/gophercloud"
	_ "github.com/rackspace/gophercloud/openstack"
	_ "github.com/rackspace/gophercloud/openstack/compute/v2/servers"
)

type StackCManager struct {
	sclient map[string]*gophercloud.ProviderClient
}

func NewStackCManager(config *goconfig.ConfigFile) (*StackCManager, error) {
	// opts, err := openstack.AuthOptionsFromEnv()
	// provider, err := openstack.AuthenticatedClient(opts)
	// client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
	// 	Region: os.Getenv("OS_REGION_NAME"),
	// })

	// return &StackCManager{
	// 	client: cilent,
	// }
	return nil, nil
}

func (dc *StackCManager) Destroy() {
}

func (s *StackCManager) AddServer(cmd *Command) *Response {
	// server, err := servers.Create(client, servers.CreateOpts{
	// 	Name:      "My new server!",
	// 	FlavorRef: "flavor_id",
	// 	ImageRef:  "image_id",
	// }).Extract()
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
