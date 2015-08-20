package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

import (
	"collectd.org/api"
	"collectd.org/exec"
	"collectd.org/network"
	"github.com/mangalaman93/nfs/docker"
	"github.com/mangalaman93/nfs/linux"
)

var clients []*network.Client

func dispatch(key string, now time.Time, value int64, prefix string) {
	vl := api.ValueList{
		Identifier: api.Identifier{
			Host:           exec.Hostname(),
			Plugin:         fmt.Sprintf("%snet", prefix),
			PluginInstance: key},
		Time:     now,
		Interval: exec.Interval(),
		Values:   []api.Value{api.Gauge(value)},
	}
	exec.Putval.Write(vl)

	for _, client := range clients {
		if err := client.Write(vl); err != nil {
			log.Printf("[WARN] unable to write to client:%s, err:%s", client, err.Error())
		}
	}
}

func sendNetUsage(interval time.Duration) {
	out, err := linux.Exec("ifconfig -s")
	now := time.Now()
	if err != nil {
		log.Printf("[WARN] unable to get system net usage: ", err.Error())
	} else {
		reader := csv.NewReader(strings.NewReader(out))
		reader.Comma = ' '
		reader.TrimLeadingSpace = true
		net_data, err := reader.ReadAll()
		if err != nil {
			log.Println("[WARN] unable to get system net usage: ", err.Error())
		} else {
			for index, row := range net_data {
				if index == 0 {
					continue
				}

				rx, err := strconv.ParseInt(row[3], 10, 64)
				if err != nil {
					log.Println("[WARN] error in converting to integer: ", err.Error())
				} else {
					dispatch(row[0], now, rx, "rx")
				}

				tx, err := strconv.ParseInt(row[7], 10, 64)
				if err != nil {
					log.Println("[WARN] error in converting to integer: ", err.Error())
				} else {
					dispatch(row[0], now, tx, "tx")
				}
			}
		}
	}

	conts, err := docker.ListContainers()
	if err != nil {
		log.Fatalln("[WARN] unable to get list of containers: ", err.Error())
	}
	for _, cont := range conts {
		net, err := docker.GetNetInUsage(cont)
		now = time.Now()
		if err != nil {
			log.Printf("[WARN] unable to get rx net usage for %s: %s\n", cont, err.Error())
		} else {
			dispatch(cont, now, net, "rx")
		}

		net, err = docker.GetNetOutUsage(cont)
		now = time.Now()
		if err != nil {
			log.Printf("[WARN] unable to get tx net usage for %s: %s\n", cont, err.Error())
		} else {
			dispatch(cont, now, net, "tx")
		}
	}

	for _, client := range clients {
		client.Flush()
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("[ERROR] %s requires more arguments!\n", os.Args[0])
	}

	addresses := strings.Fields(os.Args[1])
	clients = make([]*network.Client, len(addresses))
	index := 0
	for _, address := range addresses {
		client, err := network.Dial(address, network.ClientOptions{})
		if err != nil {
			log.Printf("[WARN] unable to connect to %s!\n", address)
			continue
		}

		clients[index] = client
		index += 1
		defer client.Close()
	}

	e := exec.NewExecutor()
	e.VoidCallback(sendNetUsage, exec.Interval())
	e.Run()
}
