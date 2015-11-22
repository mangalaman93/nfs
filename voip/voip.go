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
	vh       *VoipHandler
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

	vh, err := NewVoipHandler(config)
	if err != nil {
		sock.Close()
		return nil, err
	}

	return &VoipLine{
		database: db,
		sockfile: sockfile,
		sock:     sock,
		vh:       vh,
		quit:     make(chan interface{}),
	}, nil
}

func (v *VoipLine) Start() {
	v.vh.Start()
	go v.accept()
}

func (v *VoipLine) Stop() {
	// close the quit channel so that when we close the socket,
	// error will occur in Accept function of socket and then
	// the receive on quit will return a value. We will, then,
	// wait for the accept function to exit and stop the vh handler
	close(v.quit)
	v.sock.Close()
	v.wg.Wait()
	v.vh.Stop()
	os.Remove(v.sockfile)
	log.Println("[INFO] exiting voip loop")
}

func (v *VoipLine) GetDB() string {
	return v.database
}

// handle concurrent calls to this function
func (v *VoipLine) Update(points models.Points) {
	v.vh.UpdatePoints(points)
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

// TODO: multiple request per connection
func (v *VoipLine) handleConn(conn *net.UnixConn) {
	defer conn.Close()
	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	var req Request
	switch err := dec.Decode(&req); err {
	case nil:
		err := enc.Encode(v.vh.HandleRequest(&req))
		if err != nil {
			log.Println("[WARN] error in sending data:", err)
		}
	case io.EOF:
		log.Println("[INFO] connection with", conn.RemoteAddr(), "closed")
	default:
		log.Println("[WARN] unexpected data:", err)
	}
}
