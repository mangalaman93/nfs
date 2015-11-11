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

// state
type State struct {
	snort_queue_drops_count int
	snort_user_drops_count  int
}

var states = map[string]State{}

const (
	MAX_COUNT = 10
)

func checkState() {
	for id, s := range states {
		if s.snort_queue_drops_count > MAX_COUNT || s.snort_user_drops_count > MAX_COUNT {
			// migrate a client to another snort
			s.snort_queue_drops_count = -1 * MAX_COUNT
			s.snort_user_drops_count = -1 * MAX_COUNT
			log.Printf("[INFO] migration invoked! %s is having packet dropped\n", id)
		}
	}
}

func updateState(points models.Points) {
	for _, point := range points {
		switch point.Name() {
		case "snort_queue_drops":
			v, ok := point.Fields()["value"].(int64)
			if !ok {
				log.Println("[WARN] unknown data type!")
				continue
			}

			cont := point.Tags()["container_name"]
			s, ok := states[cont]
			if !ok {
				states[cont] = State{}
				s = states[cont]
			}

			if v > 0 {
				s.snort_queue_drops_count += 1
			} else {
				s.snort_queue_drops_count = 0
			}
			break
		case "snort_user_drops":
			v, ok := point.Fields()["value"].(int64)
			if !ok {
				log.Println("[WARN] unknown data type!")
				continue
			}

			cont := point.Tags()["container_name"]
			s, ok := states[cont]
			if !ok {
				states[cont] = State{}
				s = states[cont]
			}

			if v > 0 {
				s.snort_user_drops_count += 1
			} else {
				s.snort_user_drops_count = 0
			}
			break
		}
	}
}

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

	updateState(points)
	checkState()
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
