package main

import (
	"log"
	"time"
)

import (
	"collectd.org/api"
	"collectd.org/exec"
	"github.com/mangalaman93/nfs/docker"
	"github.com/mangalaman93/nfs/linux"
)

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
}

func main() {
	e := exec.NewExecutor()
	e.VoidCallback(sendMemUsage, exec.Interval())
	e.Run()
}
