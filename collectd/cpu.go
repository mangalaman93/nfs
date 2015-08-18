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

type ctuple struct {
	time time.Time
	cpu  int64
}

var data = make(map[string]*ctuple)

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
}

func main() {
	e := exec.NewExecutor()
	e.VoidCallback(sendCPUUsage, exec.Interval())
	e.Run()
}
