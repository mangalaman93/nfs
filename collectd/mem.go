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

var clients []*network.Client

func dispatch(key string, now time.Time, value int64) {
	vl := api.ValueList{
		Identifier: api.Identifier{
			Host:           exec.Hostname(),
			Plugin:         "mem",
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

func sendMemUsage(interval time.Duration) {
	mem, err := linux.GetMemUsage()
	now := time.Now()
	if err != nil {
		log.Printf("[WARN] unable to get system memory usage: ", err.Error())
	} else {
		dispatch("system", now, mem)
	}

	conts, err := docker.ListContainers()
	if err != nil {
		log.Fatalln("[WARN] unable to get list of containers: ", err.Error())
	}
	for _, cont := range conts {
		mem, err = docker.GetMemUsage(cont)
		now = time.Now()
		if err != nil {
			log.Printf("[WARN] unable to get memory usage for %s: %s\n", cont, err.Error())
		} else {
			dispatch(cont, now, mem)
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
	e.VoidCallback(sendMemUsage, exec.Interval())
	e.Run()
}
