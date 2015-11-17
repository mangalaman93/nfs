package voip

import (
	"errors"
)

type CCode int

const (
	CmdStartClient CCode = iota
	CmdStartServer
	CmdStartSnort
	CmdStopCont
	CmdRouteCont
)

type Command struct {
	Code   CCode
	KeyVal map[string]string
}

type Response struct {
	Result string
	Err    string
}

var (
	ErrUnknownCmd     = errors.New("Unknown command")
	ErrUnknownManager = errors.New("Inavalid container manager type")
	ErrKeyNotFound    = errors.New("All required keys not found")
)

type CManager interface {
	AddServer(cmd *Command) *Response
	AddClient(cmd *Command) *Response
	AddSnort(cmd *Command) (*Response, string)
	Stop(cmd *Command) *Response
	Route(cmd *Command) *Response
	SetShares(id string, shares int64)
	Destroy()
}

func (s *State) handleCommand(cmd *Command) *Response {
	switch cmd.Code {
	case CmdStartClient:
		return s.mger.AddClient(cmd)
	case CmdStartServer:
		return s.mger.AddServer(cmd)
	case CmdStartSnort:
		r, shares := s.mger.AddSnort(cmd)
		if r.Err != "" {
			return r
		}

		s.uchan <- r.Result
		s.uchan <- shares
		return r
	case CmdStopCont:
		return s.mger.Stop(cmd)
	case CmdRouteCont:
		return s.mger.Route(cmd)
	default:
		return &Response{Err: ErrUnknownCmd.Error()}
	}
}
