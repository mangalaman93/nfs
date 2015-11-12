package voip

import (
	"encoding/gob"
	"io"
	"log"
	"net"
	"os"

	"github.com/Unknwon/goconfig"
)

type VoipLine struct {
	sockfile string
	sock     *net.UnixListener
	quit     chan bool
}

func NewVoipLine(config *goconfig.ConfigFile) (*VoipLine, error) {
	sockfile, err := config.GetValue("VOIP", "unix_sock")
	if err != nil {
		return nil, err
	}

	sock, err := net.ListenUnix("unix", &net.UnixAddr{sockfile, "unix"})
	if err != nil {
		return nil, err
	}

	return &VoipLine{
		sockfile: sockfile,
		sock:     sock,
		quit:     make(chan bool, 1),
	}, nil
}

func (v *VoipLine) Start() {
	go v.accept()
}

func (v *VoipLine) Stop() {
	v.quit <- true
	v.sock.Close()
	<-v.quit

	os.Remove(v.sockfile)
	log.Println("[INFO] exiting voip loop")
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
		handleRequest(conn)
	}
}

func handleRequest(conn *net.UnixConn) {
	defer conn.Close()
	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	for {
		var cmd Command
		switch err := dec.Decode(&cmd); err {
		case nil:
			response := handleCommand(cmd)
			err := enc.Encode(response)
			if err != nil {
				log.Println("[WARN] error in sending voip data:", err)
			} else {
				continue
			}
		case io.EOF:
			log.Println("[INFO] connection with", conn.RemoteAddr(), "closed")
			return
		default:
			log.Println("[WARN] unexpected data:", err)
			return
		}
	}
}
