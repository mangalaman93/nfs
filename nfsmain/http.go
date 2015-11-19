package nfsmain

import (
	"compress/gzip"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/Unknwon/goconfig"
	"github.com/influxdb/influxdb/models"
)

type Handler struct {
	apps     map[string]AppLine
	dup      bool
	endpoint string
}

func NewHandler(config *goconfig.ConfigFile, apps map[string]AppLine) (*Handler, error) {
	var h *Handler

	if s, _ := config.GetSection("VOIP.DB"); s == nil {
		h = &Handler{
			apps: apps,
			dup:  false,
		}
	} else {
		ihost, err := config.GetValue("VOIP.DB", "host")
		if err != nil {
			return nil, err
		}

		iport, err := config.GetValue("VOIP.DB", "port")
		if err != nil {
			return nil, err
		}

		h = &Handler{
			apps:     apps,
			dup:      true,
			endpoint: ihost + ":" + iport,
		}
	}

	return h, nil
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("[_INFO] received data from", r.Host)
	h.duplicateRequest(r)

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
	app, ok := h.apps[database]
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
			log.Println("[_INFO] closing connection with", r.Host)
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

func (h *Handler) duplicateRequest(r *http.Request) {
}
