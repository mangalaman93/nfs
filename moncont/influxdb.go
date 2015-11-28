package main

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	influxdb "github.com/influxdb/influxdb/client"
)

const (
	DB          = "cadvisor"
	BUFFER_DUR  = 5000
	BUFFER_SIZE = 1000
)

var (
	ErrParsingAddress = errors.New("Unable to parse address")
	ErrNoDBClients    = errors.New("empty list of address")
)

type DBClient struct {
	clients  []*influxdb.Client
	machine  string
	datachan chan *influxdb.Point
	wg       sync.WaitGroup
}

func NewDBClient(addresses []string) (*DBClient, error) {
	if len(addresses) == 0 {
		return nil, ErrNoDBClients
	}

	id, err := os.Hostname()
	if err != nil {
		log.Println("[ERROR] unable to get system id", err)
		return nil, err
	}
	machine := strings.TrimSpace(string(id))

	clients := make([]*influxdb.Client, 0, len(addresses))
	for _, address := range addresses {
		fields := strings.Split(address, ":")
		if len(fields) < 2 || len(fields) == 3 || len(fields) > 4 {
			return nil, ErrParsingAddress
		}
		host, err := url.Parse(fmt.Sprintf("http://%s:%s", fields[0], fields[1]))
		if err != nil {
			return nil, err
		}

		var conf influxdb.Config
		if len(fields) == 4 {
			conf = influxdb.Config{
				URL:      *host,
				Username: fields[2],
				Password: fields[3],
			}
		} else {
			conf = influxdb.Config{URL: *host}
		}

		client, err := influxdb.NewClient(conf)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
		log.Println("[INFO] adding influxdb client:", address)
	}

	return &DBClient{
		clients:  clients,
		machine:  machine,
		datachan: make(chan *influxdb.Point, BUFFER_SIZE),
	}, nil
}

func (d *DBClient) Start() error {
	d.wg.Add(1)
	go d.sendPoints()
	return nil
}

func (d *DBClient) Stop() {
	close(d.datachan)
	d.wg.Wait()
	log.Println("[INFO] exiting influxdb routine")
}

func (d *DBClient) Write(table, contid string, fields map[string]interface{}, curtime time.Time) {
	d.datachan <- &influxdb.Point{
		Measurement: table,
		Tags: map[string]string{
			"container_name": contid,
		},
		Fields: fields,
		Time:   curtime,
	}
}

func (d *DBClient) sendPoints() {
	defer d.wg.Done()
	pts := make([]influxdb.Point, 0, 100)
	timeout := time.After(BUFFER_DUR * time.Millisecond)
	for {
		select {
		case point, more := <-d.datachan:
			if !more {
				if len(pts) != 0 {
					d.sendBatch(pts)
				}
				return
			}
			pts = append(pts, *point)
		case <-timeout:
			timeout = time.After(BUFFER_DUR * time.Millisecond)
			if len(pts) == 0 {
				continue
			}
			d.sendBatch(pts)
			pts = pts[:0]
		}
	}
}

func (d *DBClient) sendBatch(pts []influxdb.Point) {
	for _, client := range d.clients {
		client.Write(influxdb.BatchPoints{
			Points:          pts,
			Database:        DB,
			RetentionPolicy: "default",
			Tags:            map[string]string{"machine": d.machine},
		})
	}
}
