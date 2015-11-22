package nfsmain

import (
	"errors"
	"net"
	"time"
)

var (
	ErrListenerStopped = errors.New("listener is stopped")
)

type StoppableListener struct {
	*net.TCPListener
	stop chan interface{}
}

func NewStoppableListener(l *net.TCPListener) *StoppableListener {
	return &StoppableListener{
		TCPListener: l,
		stop:        make(chan interface{}),
	}
}

func (sl *StoppableListener) Stop() {
	close(sl.stop)
}

func (sl *StoppableListener) Accept() (net.Conn, error) {
	for {
		sl.SetDeadline(time.Now().Add(time.Second))
		conn, err := sl.TCPListener.Accept()
		select {
		case <-sl.stop:
			return nil, ErrListenerStopped
		default:
		}

		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() && netErr.Temporary() {
				continue
			}
		}

		return conn, err
	}
}
