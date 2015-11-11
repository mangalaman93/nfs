package nfsmain

import (
	"compress/gzip"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/influxdb/influxdb/models"
)

const (
	INFLUXDB_DB = "cadvisor"
)

var sdata State

func Listen(port string) error {
	// register handlers for collecting data over line protocol
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/write", writeHandler)

	// start listening until ctrl+c
	go http.ListenAndServe(":"+port, nil)
	return nil
}

func Stop() {
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("[WARN] unexpected request:", r)
	w.WriteHeader(http.StatusNoContent)
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
	if database != INFLUXDB_DB {
		log.Println("[_WARN] unexpected database:", database)
		writeErr(w, errors.New("database is required"))
		return
	}

	sdata.Update(points)
	sdata.Trigger()
	w.WriteHeader(http.StatusNoContent)
}

func writeErr(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
	w.Write([]byte("\n"))
}
