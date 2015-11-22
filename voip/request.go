package voip

import (
	"errors"
	"strconv"
)

var (
	ErrKeyNotFound = errors.New("All required keys not found")
	ErrIdNotExists = errors.New("container id doesn't exists")
)

const (
	ReqStartServer = iota
	ReqStartSnort
	ReqStartClient
	ReqStopCont
	ReqRouteCont
)

type Request struct {
	Code   int
	KeyVal map[string]string
}

type Response struct {
	Result string
	Err    string
}

func (vh *VoipHandler) addServer(req *Request) *Response {
	kv := req.KeyVal
	host, ok1 := kv["host"]
	sshares, ok2 := kv["shares"]
	if !ok1 || !ok2 {
		return &Response{Err: ErrKeyNotFound.Error()}
	}

	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	node, err := vh.cmgr.StartServer(host, shares)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	vh.anodes[node.id] = node
	return &Response{Result: node.id}
}

func (vh *VoipHandler) addSnort(req *Request) *Response {
	kv := req.KeyVal
	host, ok1 := kv["host"]
	sshares, ok2 := kv["shares"]
	if !ok1 || !ok2 {
		return &Response{Err: ErrKeyNotFound.Error()}
	}

	shares, err := strconv.ParseInt(sshares, 10, 64)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	node, err := vh.cmgr.StartSnort(host, shares)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	vh.addMCont(node, shares)
	return &Response{Result: node.id}
}

func (vh *VoipHandler) addClient(req *Request) *Response {
	kv := req.KeyVal
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
	server, ok := vh.anodes[serverid]
	if !ok {
		return &Response{Err: ErrIdNotExists.Error()}
	}

	node, err := vh.cmgr.StartClient(host, shares, server.ip)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	vh.anodes[node.id] = node
	return &Response{Result: node.id}
}

func (vh *VoipHandler) stopCont(req *Request) *Response {
	kv := req.KeyVal
	contid, ok := kv["cont"]
	if !ok {
		return &Response{Err: ErrKeyNotFound.Error()}
	}

	node, ok := vh.anodes[contid]
	if ok {
		vh.cmgr.StopCont(node)
	} else {
		mnode, ok := vh.mnodes[contid]
		if ok {
			vh.delMCont(mnode)
			return &Response{Err: ErrIdNotExists.Error()}
		}
	}

	return nil
}

func (vh *VoipHandler) route(req *Request) *Response {
	kv := req.KeyVal
	client, ok1 := kv["client"]
	server, ok2 := kv["server"]
	router, ok3 := kv["router"]
	if !ok1 || !ok2 || !ok3 {
		return &Response{Err: ErrKeyNotFound.Error()}
	}

	cnode, ok1 := vh.anodes[client]
	snode, ok3 := vh.anodes[server]
	rnode, ok2 := vh.mnodes[router]
	if !ok1 || !ok2 || !ok3 {
		return &Response{Err: ErrIdNotExists.Error()}
	}

	err := vh.cmgr.Route(cnode, snode, rnode.Node)
	return &Response{Err: err.Error()}
}

func (vh *VoipHandler) addMCont(node *Node, shares int64) {
	vh.Lock()
	defer vh.Unlock()
	vh.mnodes[node.id] = NewMContainer(node, vh.step_length, vh.period_length, shares, vh.reference)
}

func (vh *VoipHandler) delMCont(node *MContainer) {
	vh.Lock()
	defer vh.Unlock()
	delete(vh.mnodes, node.id)
}
