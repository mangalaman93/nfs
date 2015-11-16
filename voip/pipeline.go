package voip

import (
	"errors"
)

var (
	ErrIdAlreadyExists = errors.New("container id already exists")
	ErrIdNotExists     = errors.New("container id doesn't exists")
	ErrInvalidPipe     = errors.New("invalid pipe state")
	ErrOrphanChild     = errors.New("Current node will have orphans childs")
)

type Node struct {
	id     string
	ip4    string
	mac    string
	host   string
	parent map[string]*Node
	child  map[string]*Node
}

type PipeLine struct {
	root      *Node
	idToNode  map[string]*Node // only the nodes in the tree
	restNodes map[string]*Node // nodes which has not been added to tree yet
}

func newNode(id, ip4, mac, host string) *Node {
	return &Node{
		id:     id,
		ip4:    ip4,
		mac:    mac,
		host:   host,
		parent: make(map[string]*Node),
		child:  make(map[string]*Node),
	}
}

var (
	RootNode = newNode("", "", "", "")
)

// we start with dummy root node
func NewPipeLine() *PipeLine {
	return &PipeLine{
		root:      RootNode,
		idToNode:  make(map[string]*Node),
		restNodes: make(map[string]*Node),
	}
}

// we simply add the node in the list of rest of the nodes
func (p *PipeLine) NewNode(id, ip4, mac, host string) error {
	_, ok1 := p.idToNode[id]
	_, ok2 := p.restNodes[id]
	if ok1 || ok2 {
		return ErrIdAlreadyExists
	}

	node := newNode(id, ip4, mac, host)
	p.restNodes[id] = node
	return nil
}

// adds node in the tree
func (p *PipeLine) AddNode(cur, parent string) error {
	me, ok1 := p.restNodes[cur]
	np, ok2 := p.idToNode[parent]
	if !ok1 || !ok2 {
		return ErrIdNotExists
	}

	_, ok1 = p.idToNode[cur]
	_, ok2 = p.restNodes[parent]
	if ok1 || ok2 {
		return ErrInvalidPipe
	}

	_, ok1 = np.child[cur]
	_, ok2 = me.parent[parent]
	if ok1 || ok2 {
		return ErrIdAlreadyExists
	}

	delete(p.restNodes, cur)
	np.child[cur] = me
	me.parent[parent] = np
	p.idToNode[cur] = me
	return nil
}

// TODO: modifies the tree
func (p *PipeLine) ModNode(cur, nparent string) error {
	return nil
}

// we throw error if a child will become
// orphan on deleting current node
func (p *PipeLine) DelNode(cur string) error {
	_, ok := p.restNodes[cur]
	if ok {
		_, ok1 := p.idToNode[cur]
		if ok1 {
			return ErrInvalidPipe
		}

		delete(p.restNodes, cur)
		return nil
	}

	me, ok := p.idToNode[cur]
	if !ok {
		return ErrIdNotExists
	}

	if checkOrphan(me) {
		return ErrOrphanChild
	}

	for _, node := range me.parent {
		delete(node.child, cur)
	}
	delete(p.idToNode, cur)
	return nil
}

// TODO
func (p *PipeLine) String() string {
	return ""
}

func (p *PipeLine) GetIPAddress(cur string) (string, error) {
	me, ok := p.restNodes[cur]
	if ok {
		return me.ip4, nil
	}

	me, ok = p.idToNode[cur]
	if ok {
		return me.ip4, nil
	}

	return "", ErrIdNotExists
}

func (p *PipeLine) GetMacAddress(cur string) (string, error) {
	me, ok := p.restNodes[cur]
	if ok {
		return me.mac, nil
	}

	me, ok = p.idToNode[cur]
	if ok {
		return me.mac, nil
	}

	return "", ErrIdNotExists
}

func (p *PipeLine) GetHost(cur string) (string, error) {
	me, ok := p.restNodes[cur]
	if ok {
		return me.host, nil
	}

	me, ok = p.idToNode[cur]
	if ok {
		return me.host, nil
	}

	return "", ErrIdNotExists
}

func checkOrphan(me *Node) bool {
	for _, node := range me.child {
		if len(node.parent) <= 1 {
			return true
		}
	}

	return false
}