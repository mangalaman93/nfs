package voip

import (
	"encoding/gob"
	"io"
	"log"
	"net"
	"os"
	"sync"

	"github.com/Unknwon/goconfig"
	"github.com/influxdb/influxdb/models"
)

type VoipLine struct {
	database string
	sockfile string
	sock     *net.UnixListener
	state    *State
	quit     chan interface{}
	wg       sync.WaitGroup
}

func NewVoipLine(config *goconfig.ConfigFile) (*VoipLine, error) {
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

	state, err := NewState(config)
	if err != nil {
		sock.Close()
		return nil, err
	}

	return &VoipLine{
		database: db,
		sockfile: sockfile,
		sock:     sock,
		state:    state,
		quit:     make(chan interface{}),
	}, nil
}

func (v *VoipLine) Start() {
	v.state.Init()
	v.accept()
}

func (v *VoipLine) Stop() {
	close(v.quit)
	v.sock.Close()
	v.state.Destroy()
	v.wg.Wait()

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
	v.wg.Add(1)
	defer v.wg.Done()
	log.Println("[INFO] listening voip commands on", v.sockfile)

	for {
		conn, err := v.sock.AcceptUnix()
		if err != nil {
			select {
			case <-v.quit:
				return
			default:
				log.Println("[WARN] error accepting:", err)
				continue
			}
		}

		log.Println("[INFO] Received request from", conn.RemoteAddr())
		v.handleConn(conn)
	}
}

func (v *VoipLine) handleConn(conn *net.UnixConn) {
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
