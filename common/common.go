package common

import (
	"encoding/gob"
	"fmt"
	"net"
)

const (
	CREATE_CONTAINER = iota
	STOP_CONTAINER
	INC_CPU
	DEC_CPU
	INC_MEM
	DEC_MEM
)

type Capacity struct {
	Ncpu   int
	Memory int64
}

type Host struct {
	Endpoint net.Addr
	Enc      *gob.Encoder
	Dec      *gob.Decoder
	Cap      Capacity
}

type Cmd struct {
	Code int
	Args string
}

func (c *Capacity) String() string {
	return fmt.Sprintf("{Ncpu:%d, Memory:%d}", c.Ncpu, c.Memory)
}

func (h *Host) String() string {
	return fmt.Sprintf("{ip:(%s), capacity:%s}", h.Endpoint, h.Cap)
}

func (c *Cmd) String() string {
	return fmt.Sprintf("{Code:%d, Args:(%s)}", c.Code, c.Args)
}
