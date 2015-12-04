package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mangalaman93/nfs/client"
)

const (
	localhost = "local"
	cpath     = "/home/ubuntu/nfs/.voip.conf"
)

func main() {
	vc, err := client.NewVoipClient(cpath)
	if err != nil {
		panic(err)
	}
	defer vc.Close()

	server, err := vc.AddServer(localhost, 1024)
	if err != nil {
		panic(err)
	}
	defer func() { log.Println("stopping server, err:", vc.Stop(server)) }()
	log.Println("started server:", server)

	snort, err := vc.AddSnort(localhost, 1024)
	if err != nil {
		panic(err)
	}
	defer func() { log.Println("stopping snort, err:", vc.Stop(snort)) }()
	log.Println("started snort:", snort)

	client, err := vc.AddClient(localhost, 1024, server)
	if err != nil {
		panic(err)
	}
	defer func() { log.Println("stopping client, err:", vc.Stop(client)) }()
	log.Println("started client:", client)

	err = vc.Route(client, snort, server)
	if err != nil {
		panic(err)
	}
	log.Println("routes are setup")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	time.Sleep(10 * time.Second)
	flag := false
	for rate := 500; rate < 5000; rate += 500 {
		vc.SetRate(client, rate)
		log.Println("set rate to", rate)

		timeout := time.After(60 * time.Second)
		select {
		case <-sigs:
			flag = true
			break
		case <-timeout:
		}

		if flag {
			break
		}
	}

	log.Println("begining cleanup!")
}
