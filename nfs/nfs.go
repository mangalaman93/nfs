package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
)

import (
	"github.com/VividCortex/godaemon"
	_ "github.com/influxdb/influxdb/models"
)

const (
	LOG_FILE = "nfs.log"
)

func parseArgs(daemonize *bool, port *string) {
	flag.BoolVar(daemonize, "d", false, "daemonize")
	flag.StringVar(port, "p", "8086", "port")
	flag.Parse()
}

func main() {
	var logfile *os.File
	var err error
	var port string

	daemonize := true
	if godaemon.Stage() == godaemon.StageParent {
		parseArgs(&daemonize, &port)

		if daemonize {
			logfile, err = os.OpenFile(LOG_FILE, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				fmt.Println("[ERROR] error opening log file:", err)
				os.Exit(1)
			}

			err = syscall.Flock(int(logfile.Fd()), syscall.LOCK_EX)
			if err != nil {
				fmt.Println("[ERROR] error acquiring lock to log file:", err)
				os.Exit(1)
			}
		}
	}

	if daemonize {
		_, _, err = godaemon.MakeDaemon(&godaemon.DaemonAttr{Files: []**os.File{&logfile}})
		if err != nil {
			fmt.Println("[ERROR] error daemonizing:", err)
			os.Exit(1)
		}

		defer logfile.Close()
		log.SetOutput(logfile)
		parseArgs(&daemonize, &port)
	}

	log.SetFlags(log.LstdFlags)
	log.Println("#################### BEGIN OF LOG ##########################")

	// listening to incoming connections
	conn, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalln("[ERROR] listening:", err)
	}
	defer conn.Close()
	log.Println("[_INFO] listening on", port)

	for {
		client, err := conn.Accept()
		if err != nil {
			log.Println("[_WARN] error accepting: ", err.Error())
			continue
		}
		log.Printf("[_INFO] Received request from %s\n", client.RemoteAddr())

		reader := bufio.NewReader(client)
		line, isPrefix, err := reader.ReadLine()
		for err == nil && !isPrefix {
			log.Println(string(line))
			line, isPrefix, err = reader.ReadLine()
		}
		if isPrefix {
			log.Println("buffer size is too small!")
		} else {
			log.Println(err)
		}
	}
}
