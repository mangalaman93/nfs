package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
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

func writeHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("[WARN] unexpected request:", r)
	w.WriteHeader(http.StatusNoContent)
}

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
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/write", writeHandler)
	http.ListenAndServe(":"+port, nil)
}
