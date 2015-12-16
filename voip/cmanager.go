package voip

import (
	"errors"
)

type CManager interface {
	Setup() error
	Destroy()
	StartServer(host string, shares int64) (*Node, error)
	StartSnort(host string, shares int64) (*Node, error)
	StartClient(host string, shares int64, serverip string) (*Node, error)
	StopCont(node *Node) error
	Route(cnode, rnode, snode *Node) error
	SetShares(node *Node, shares int64) error
}

const (
	IMG_SIPP       = "mangalaman93/sipp"
	IMG_SNORT      = "mangalaman93/snort"
	IMG_CADVISOR   = "mangalaman93/cadvisor"
	IMG_MONCONT    = "mangalaman93/moncont"
	SIPP_BUFF_SIZE = "1048576"
	CPU_PERIOD     = 100000
	BUF_DURATION   = "5s"
)

var (
	ErrHostNotFound = errors.New("Host not found")
	ErrNoHosts      = errors.New("error while finding host list")
)
