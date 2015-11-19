package nfsmain

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
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

func (h Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r := h.duplicateRequest(req)
	precision := r.FormValue("precision")
	if precision == "" {
		precision = "n"
	}

	// Handle gzip decoding of the body
	body := r.Body
	if r.Header.Get("Content-encoding") == "gzip" {
		unzip, err := gzip.NewReader(r.Body)
		if err != nil {
			log.Println("[WARN] unable to unzip body:", err)
			writeErr(w, err)
			return
		}
		body = unzip
	}
	defer body.Close()

	database := r.FormValue("db")
	app, ok := h.apps[database]
	if !ok {
		log.Println("[WARN] unregistered database:", database)
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
			log.Println("[INFO] closing connection with", r.Host)
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

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

// ref:https://github.com/chrislusf/teeproxy/blob/master/teeproxy.go
func (h *Handler) duplicateRequest(r *http.Request) *http.Request {
	buf1 := new(bytes.Buffer)
	buf2 := new(bytes.Buffer)
	w := io.MultiWriter(buf1, buf2)
	io.Copy(w, r.Body)
	defer r.Body.Close()

	request := &http.Request{
		Method:        r.Method,
		URL:           r.URL,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        r.Header,
		Body:          nopCloser{buf1},
		Host:          r.Host,
		ContentLength: r.ContentLength,
	}
	drequest := &http.Request{
		Method:        r.Method,
		URL:           r.URL,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        r.Header,
		Body:          nopCloser{buf2},
		Host:          r.Host,
		ContentLength: r.ContentLength,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println("[WARN] Recovered in duplicateRequest", r)
			}
		}()

		con, err := net.DialTimeout("tcp", h.endpoint, time.Duration(1*time.Second))
		if err != nil {
			log.Println("[WARN] unable to connect to influxdb database")
			return
		}
		hcon := httputil.NewClientConn(con, nil)
		defer hcon.Close()

		err = hcon.Write(request)
		if err != nil {
			log.Println("[WARN] unable to write to influxdb database")
			return
		}
		hcon.Read(request)
	}()

	return drequest
}
