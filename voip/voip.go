package voip

import (
	"encoding/gob"
	"io"
	"log"
	"net"
	"os"

	"github.com/Unknwon/goconfig"
	"github.com/influxdb/influxdb/models"
)

type VoipLine struct {
	database string
	sockfile string
	sock     *net.UnixListener
	state    *State
	quit     chan bool
}

func NewVoipLine(config *goconfig.ConfigFile) (*VoipLine, error) {
	undo := true

	state, err := NewState(config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if undo {
			state.Destroy()
		}
	}()

	sockfile, err := config.GetValue("VOIP", "unix_sock")
	if err != nil {
		return nil, err
	}
	db, err := config.GetValue("VOIP", "db")
	if err != nil {
		return nil, err
	}
	sock, err := net.ListenUnix("unix", &net.UnixAddr{sockfile, "unix"})
	if err != nil {
		return nil, err
	}

	undo = false
	return &VoipLine{
		database: db,
		sockfile: sockfile,
		sock:     sock,
		state:    state,
		quit:     make(chan bool, 1),
	}, nil
}

func (v *VoipLine) Start() {
	v.accept()
}

func (v *VoipLine) Stop() {
	v.quit <- true
	v.sock.Close()
	v.state.Destroy()
	<-v.quit

	os.Remove(v.sockfile)
	log.Println("[INFO] exiting voip loop")
}

func (v *VoipLine) GetDB() string {
	return v.database
}

func (v *VoipLine) Update(points models.Points) {
	v.state.Update(points)
}

func (v *VoipLine) accept() {
	log.Println("[INFO] listening voip commands on", v.sockfile)

	for {
		conn, err := v.sock.AcceptUnix()
		if err != nil {
			select {
			case <-v.quit:
				close(v.quit)
				return
			default:
				log.Println("[WARN] error accepting:", err)
				continue
			}
		}

		log.Println("[INFO] Received request from", conn.RemoteAddr())
		v.handleRequest(conn)
	}
}

func (v *VoipLine) handleRequest(conn *net.UnixConn) {
	defer conn.Close()
	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	var cmd Request
	switch err := dec.Decode(&cmd); err {
	case nil:
		response := v.state.handleRequest(&cmd)
		err := enc.Encode(response)
		if err != nil {
			log.Println("[WARN] error in sending voip data:", err)
		}
	case io.EOF:
		log.Println("[INFO] connection with", conn.RemoteAddr(), "closed")
	default:
		log.Println("[WARN] unexpected data:", err)
	}
}
