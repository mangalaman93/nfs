package voip

import (
	"errors"
)

type CCode int

const (
	StartClient CCode = iota
	StartServer
	StartSnort
	RouteCont
)

type Command struct {
	Code   CCode
	KeyVal map[string]string
}

type Response struct {
	result string
	err    error
}

var (
	ErrInvalidArgType     = errors.New("invalid argument type")
	ErrInvalidManagerType = errors.New("Inavalid container manager type")
)

type CManager interface {
	AddServer(cmd *Command) *Response
	AddClient(cmd *Command) *Response
	AddSnort(cmd *Command) *Response
	Stop(cmd *Command) *Response
	Route(cmd *Command) *Response
}

func (s *State) handleCommand(cmd *Command) *Response {
	switch cmd.Code {
	case StartClient:
		return s.mger.AddClient(cmd)
	case StartServer:
		return s.mger.AddServer(cmd)
	case StartSnort:
		return s.mger.AddSnort(cmd)
	case RouteCont:
		return s.mger.Route(cmd)
	default:
		return &Response{err: ErrInvalidArgType}
	}
}
