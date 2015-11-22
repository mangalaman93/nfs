package voip

type Node struct {
	id   string
	ip   string
	mac  string
	host string
}

func NewNode(id, ip, mac, host string) *Node {
	return &Node{
		id:   id,
		ip:   ip,
		mac:  mac,
		host: host,
	}
}
