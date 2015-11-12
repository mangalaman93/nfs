package nfsmain

import (
	"compress/gzip"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/influxdb/influxdb/models"
)

func ListenLine(port string, apps map[string]AppLine) {
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/write", writeHandler)
	http.ListenAndServe(":"+port, nil)
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("[_WARN] unexpected request:", r)
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

	database := r.FormValue("db")
	app, ok := apps[database]
	if !ok {
		log.Println("[_WARN] unregistered database:", database)
		w.WriteHeader(http.StatusNoContent)
		return
	}

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

	app.Update(points)
	w.WriteHeader(http.StatusNoContent)
}

func writeErr(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
	w.Write([]byte("\n"))
}
