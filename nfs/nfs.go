package main

import (
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"
)

import (
	"github.com/VividCortex/godaemon"
	"github.com/influxdb/influxdb/models"
)

const (
	LOG_FILE  = "nfs.log"
	INFLUX_DB = "cadvisor"
)

func writeErr(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
	w.Write([]byte("\n"))
}

func writeHandler(w http.ResponseWriter, r *http.Request) {
	precision := r.FormValue("precision")
	if precision == "" {
		precision = "n"
	}

	// Handle gzip decoding of the body
	body := r.Body
	if r.Header.Get("Content-encoding") == "gzip" {
		unzip, err := gzip.NewReader(r.Body)
		if err != nil {
			log.Println("[_WARN] unable to unzip body:", err)
			writeErr(w, err)
			return
		}
		body = unzip
	}
	defer body.Close()

	data, err := ioutil.ReadAll(body)
	if err != nil {
		writeErr(w, err)
		return
	}

	points, err := models.ParsePointsWithPrecision(data, time.Now().UTC(), precision)
	if err != nil {
		if err.Error() == "EOF" {
			log.Println("[_INFO] closing connection!")
			w.WriteHeader(http.StatusOK)
			return
		}
		writeErr(w, err)
		return
	}

	database := r.FormValue("db")
	if database != INFLUX_DB {
		log.Println("[_WARN] unexpected database:", database)
		writeErr(w, errors.New("database is required"))
		return
	}

	log.Println(points)
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
