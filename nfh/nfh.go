package main

import (
	"encoding/gob"
	"flag"
	"log"
	"net"
	"runtime"
)

import "github.com/mangalaman93/nfs/common"

func main() {
	// setting up log flags
	log.SetFlags(log.Lshortfile)

	// command line flags
	port := flag.String("port", "8080", "listen for nfh")
	host := flag.String("host", "0.0.0.0", "NFS ip address")
	id := flag.String("id", "nfh", "id for this NFH")
	flag.Parse()

	// connecting to NFS
	conn, err := net.Dial("tcp", *host+":"+*port)
	if err != nil {
		log.Fatalln("[ERROR] unable to reach NFS, ", err.Error())
	}
	log.Println("[INFO] connected to NFS")

	// close the listener when the application closes
	defer conn.Close()

	// gob encoder-decoder for client
	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	// sending id
	err = enc.Encode(*id)
	if err != nil {
		log.Fatalln("[ERROR] error encoding id, ", err.Error())
	}

	// sending capacity
	err = enc.Encode(common.Capacity{runtime.NumCPU(), common.TotalMem()})
	if err != nil {
		log.Fatalln("[ERROR] error encoding capacity, ", err.Error())
	}

	for {
		var cmd common.Cmd
		dec.Decode(&cmd)
		log.Println("[INFO] received command: ", cmd)
	}
}
