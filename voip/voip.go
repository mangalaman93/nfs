package voip

import (
	"encoding/gob"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Unknwon/goconfig"
	"github.com/influxdb/influxdb/models"
)

type VoipLine struct {
	database string
	sockfile string
	sock     *net.UnixListener
	vh       *VoipHandler
	quit     chan bool
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

	vh, err := NewVoipHandler(config)
	if err != nil {
		return nil, err
	}

	return &VoipLine{
		database: db,
		sockfile: sockfile,
		sock:     nil,
		vh:       vh,
		quit:     make(chan bool),
	}, nil
}

func (v *VoipLine) Start() error {
	var err error
	v.sock, err = net.ListenUnix("unix", &net.UnixAddr{v.sockfile, "unix"})
	if err != nil {
		return err
	}

	err = v.vh.Start()
	if err != nil {
		v.sock.Close()
		return err
	}

	go v.accept()
	return nil
}

func (v *VoipLine) Stop() {
	// close the quit channel so that when we close the socket,
	// error will occur in Accept function of socket and then
	// the receive on quit will return a value. We will, then,
	// wait for the accept function to exit and stop the vh handler
	close(v.quit)
	v.wg.Wait()
	v.sock.Close()
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
		v.sock.SetDeadline(time.Now().Add(time.Second))
		conn, err := v.sock.AcceptUnix()
		if err != nil {
			select {
			case <-v.quit:
				return
			default:
				neterr, ok := err.(net.Error)
				if !ok || !neterr.Timeout() {
					log.Println("[WARN] error accepting:", err)
				}
				continue
			}
		}

		log.Println("[INFO] Received request from", conn.RemoteAddr())
		go v.handleConn(conn)
	}
}

func (v *VoipLine) handleConn(conn *net.UnixConn) {
	v.wg.Add(1)
	defer v.wg.Done()
	defer conn.Close()
	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	var req Request
	for {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		err := dec.Decode(&req)

		// ensure that we are not expected to exit
		select {
		case <-v.quit:
			return
		default:
		}

		neterr, ok := err.(net.Error)
		switch {
		case ok && neterr.Timeout():
			break
		case err == nil:
			err := enc.Encode(v.vh.HandleRequest(&req))
			if err != nil {
				log.Println("[WARN] error in sending data:", err)
			}
		case err == io.EOF:
			log.Println("[INFO] connection with", conn.RemoteAddr(), "closed")
			return
		default:
			log.Println("[WARN] unexpected data:", err)
		}
	}
}
