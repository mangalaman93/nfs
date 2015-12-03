package voip

import (
	"errors"
	"net"
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
	ReqSetRate
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
		delete(vh.anodes, node.id)
	} else {
		mnode, ok := vh.mnodes[contid]
		if ok {
			vh.delMCont(mnode)
			return &Response{Err: ErrIdNotExists.Error()}
		}
	}

	return &Response{}
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
	rcont, ok2 := vh.mnodes[router]
	snode, ok3 := vh.anodes[server]
	if !ok1 || !ok2 || !ok3 {
		return &Response{Err: ErrIdNotExists.Error()}
	}

	err := vh.cmgr.Route(cnode, rcont.node, snode)
	if err != nil {
		return &Response{Err: err.Error()}
	} else {
		return &Response{}
	}
}

func (vh *VoipHandler) setRate(req *Request) *Response {
	kv := req.KeyVal
	client, ok1 := kv["client"]
	rate, ok2 := kv["rate"]
	if !(ok1 && ok2) {
		return &Response{Err: ErrKeyNotFound.Error()}
	}

	cnode, ok1 := vh.anodes[client]
	if !ok1 {
		return &Response{Err: ErrIdNotExists.Error()}
	}
	irate, err := strconv.ParseInt(rate, 10, 32)
	if err != nil {
		return &Response{Err: err.Error()}
	}

	err = vh.setClientRate(cnode, int(irate))
	if err != nil {
		return &Response{Err: err.Error()}
	} else {
		return &Response{}
	}
}

func (vh *VoipHandler) addMCont(node *Node, shares int64) {
	vh.mnodes[node.id] = NewMContainer(node, vh.step_length, vh.period_length, shares, vh.reference)
}

func (vh *VoipHandler) delMCont(mcont *MContainer) {
	delete(vh.mnodes, mcont.node.id)
}

func (vh *VoipHandler) setClientRate(cnode *Node, rate int) error {
	addr, err := net.ResolveUDPAddr("udp", cnode.ip+":8888")
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte("cset rate " + strconv.Itoa(rate)))
	if err != nil {
		return err
	}
	return nil
}
