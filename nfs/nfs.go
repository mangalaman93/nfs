package main

import (
	"encoding/gob"
	"flag"
	"log"
	"net"
    "os"
	"sync"
)

import (
    "github.com/mangalaman93/nfs/common"
    "collectd.org/network"
    "collectd.org/format"
)

// data structures
var hosts = map[string]common.Host{}
var hlock sync.Mutex

func aggregator() {
    // listen on 8097 for collectd data
    cserver := &network.Server {
        Addr:   net.JoinHostPort("localhost", "8097"),
        Writer: format.NewPutval(os.Stdout),
    }

    // blocks
    log.Fatal(cserver.ListenAndWrite())
}

func main() {
	// setting up log flags
	log.SetFlags(log.Lshortfile)

	// command line flags
	port := flag.String("port", "8080", "listen for nfh")
	flag.Parse()

	// listening to incoming connections from NFHs
	conn, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatalln("[ERROR] listening:", err.Error())
	}

	// close the listener when the application closes
	defer conn.Close()

    // create aggregator thread
    go aggregator()

	// accept connections
	for {
		client, err := conn.Accept()
		if err != nil {
			log.Println("[WARN] error accepting: ", err.Error())
			continue
		}

		// log an incoming connection request
		log.Printf("[INFO] Received request from %s\n", client.RemoteAddr())

		// gob encoder-decoder for client
		enc := gob.NewEncoder(client)
		dec := gob.NewDecoder(client)

		// receive NFH id
		var id string
		err = dec.Decode(&id)
		if err != nil {
			log.Println("[ERROR] unexpected data,", err.Error())
			client.Close()
			continue
		}

		// receieve capacity
		var capacity common.Capacity
		err = dec.Decode(&capacity)
		if err != nil {
			log.Println("[ERROR] unexpected data,", err.Error())
			client.Close()
			continue
		}

		// storing host
		hlock.Lock()
		hosts[id] = common.Host{client.RemoteAddr(), enc, dec, capacity}
		hlock.Unlock()
		log.Printf("added client with id %s\n", id)
	}
}
