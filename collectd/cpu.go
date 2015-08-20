package main

import (
	"log"
	"os"
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

type ctuple struct {
	time time.Time
	cpu  int64
}

var data = make(map[string]*ctuple)
var clients []*network.Client

func dispatch(key string, now time.Time, value float64) {
	vl := api.ValueList{
		Identifier: api.Identifier{
			Host:           exec.Hostname(),
			Plugin:         "cpu",
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

func sendCPUUsage(interval time.Duration) {
	cpu, err := linux.GetCPUUsage()
	now := time.Now()
	if err != nil {
		log.Printf("[WARN] unable to get system cpu usage: ", err.Error())
	} else {
		ctup, ok := data["system"]
		if ok {
			avg := float64(cpu-ctup.cpu) * 1000000000 / float64(now.Sub(ctup.time))
			ctup.time = now
			ctup.cpu = cpu
			dispatch("system", now, avg)
		} else {
			data["system"] = &ctuple{time: now, cpu: cpu}
		}
	}

	conts, err := docker.ListContainers()
	if err != nil {
		log.Fatalln("[WARN] unable to get list of containers: ", err.Error())
	}
	for _, cont := range conts {
		cpu, err = docker.GetCPUUsage(cont)
		now = time.Now()
		if err != nil {
			log.Printf("[WARN] unable to get cpu usage for %s: %s\n", cont, err.Error())
			continue
		}

		ctup, ok := data[cont]
		if ok {
			avg := float64(cpu-ctup.cpu) * 1000000000 / float64(now.Sub(ctup.time))
			ctup.time = now
			ctup.cpu = cpu
			dispatch(cont, now, avg)
		} else {
			data[cont] = &ctuple{time: now, cpu: cpu}
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
	e.VoidCallback(sendCPUUsage, exec.Interval())
	e.Run()
}
