package voip

import (
	"encoding/gob"
	"io"
	"log"
	"net"
	"os"
)

var (
	sock_file string
	sock      *net.UnixListener = nil
	quit      chan bool         = nil
)

func Listen(file string) error {
	sock_file = file

	// listen on unix socket
	var err error
	sock, err = net.ListenUnix("unix", &net.UnixAddr{sock_file, "unix"})
	if err != nil {
		return err
	}
	log.Println("[INFO] listening voip commands on", sock_file)

	// init quit channel
	quit = make(chan bool, 1)

	go accept()
	return nil
}

func Stop() {
	if quit == nil {
		return
	}

	// communicating with accept loop
	quit <- true
	sock.Close()
	<-quit

	os.Remove(sock_file)
	log.Println("[INFO] exiting voip loop")
}

func accept() {
	for {
		conn, err := sock.AcceptUnix()
		if err != nil {
			select {
			case <-quit:
				close(quit)
				return
			default:
				log.Println("[WARN] error accepting:", err)
			}

			continue
		}

		log.Println("[INFO] Received request from", conn.RemoteAddr())
		enc := gob.NewEncoder(conn)
		dec := gob.NewDecoder(conn)

		for {
			var cmd Command
			switch dec.Decode(&cmd) {
			case nil:
				response, err := handleCommand(cmd)
				if err != nil {
					log.Println("[WARN] error in handling voip command:", err)
				}

				err = enc.Encode(response)
				if err != nil {
					log.Println("[WARN] error in sending voip data:", err)
				} else {
					continue
				}
			case io.EOF:
				log.Println("[INFO] connection with", conn.RemoteAddr(), "closed")
			default:
				log.Println("[ERROR] unexpected data:", err)
			}

			break
		}

		conn.Close()
	}
}
