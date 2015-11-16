package voip

import (
	"errors"
)

type CCode int

const (
	StartClient CCode = iota
	StartServer
	StartSnort
	StopCont
	RouteCont
)

type Command struct {
	Code   CCode
	KeyVal map[string]string
}

type Response struct {
	Result string
	Err    error
}

var (
	ErrInvalidArgType     = errors.New("Invalid argument type")
	ErrInvalidManagerType = errors.New("Inavalid container manager type")
	ErrKeyNotFound        = errors.New("All required keys not found")
)

type CManager interface {
	AddServer(cmd *Command) *Response
	AddClient(cmd *Command) *Response
	AddSnort(cmd *Command) *Response
	Stop(cmd *Command) *Response
	Route(cmd *Command) *Response
	Destroy()
}

func (s *State) handleCommand(cmd *Command) *Response {
	switch cmd.Code {
	case StartClient:
		return s.mger.AddClient(cmd)
	case StartServer:
		return s.mger.AddServer(cmd)
	case StartSnort:
		r := s.mger.AddSnort(cmd)
		if r.Err != nil {
			return r
		}

		s.uchan <- r.Result
		return r
	case StopCont:
		return s.mger.Stop(cmd)
	case RouteCont:
		return s.mger.Route(cmd)
	default:
		return &Response{Err: ErrInvalidArgType}
	}
}
