package voip

type CManager interface {
	StartServer(host string, shares int64) (*Node, error)
	StartSnort(host string, shares int64) (*Node, error)
	StartClient(host string, shares int64, serverip string) (*Node, error)
	StopCont(node *Node) error
	Route(cnode, rnode, snode *Node) error
	SetShares(node *Node, shares int64) error
	Destroy()
}
