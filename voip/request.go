package voip

import (
	"errors"
)

const (
	ReqStartClient = iota
	ReqStartServer
	ReqStartSnort
	ReqStopCont
	ReqRouteCont

	reqNewMContainer
	reqDelMContainer
)

var (
	ErrUnknownReq     = errors.New("Unknown request")
	ErrUnknownManager = errors.New("Inavalid container manager type")
	ErrKeyNotFound    = errors.New("All required keys not found")
)

type Request struct {
	Code   int
	KeyVal map[string]string
}

type Response struct {
	Result string
	Err    string
}

type CManager interface {
	AddServer(req *Request) *Response
	AddClient(req *Request) *Response
	AddSnort(req *Request) (*Response, string)
	Stop(req *Request) *Response
	Route(req *Request) *Response
	SetShares(id string, shares int64)
	Destroy()
}

func (s *State) handleRequest(req *Request) *Response {
	switch req.Code {
	case ReqStartClient:
		return s.mger.AddClient(req)
	case ReqStartServer:
		return s.mger.AddServer(req)
	case ReqStartSnort:
		resp, shares := s.mger.AddSnort(req)
		if resp.Err != "" {
			return resp
		}
		s.rchan <- &Request{
			Code: reqNewMContainer,
			KeyVal: map[string]string{
				"id":     resp.Result,
				"shares": shares,
			},
		}
		return resp
	case ReqStopCont:
		// TODO: we send request for all containers even while it may not be monitored
		kv := req.KeyVal
		_, ok := kv["cont"]
		if !ok {
			return &Response{Err: ErrKeyNotFound.Error()}
		}
		s.rchan <- &Request{
			Code:   reqDelMContainer,
			KeyVal: map[string]string{"id": req.KeyVal["cont"]},
		}
		return s.mger.Stop(req)
	case ReqRouteCont:
		return s.mger.Route(req)
	default:
		return &Response{Err: ErrUnknownReq.Error()}
	}
}
