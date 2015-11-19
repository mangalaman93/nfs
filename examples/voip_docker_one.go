package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mangalaman93/nfs/client"
)

func main() {
	vc, err := client.NewVoipClient("/home/ubuntu/nfs/.voip.conf")
	if err != nil {
		panic(err)
	}
	defer vc.Close()

	server, err := vc.AddServer(1024)
	if err != nil {
		panic(err)
	}
	defer vc.Stop(server)
	fmt.Println("started server:", server)

	snort, err := vc.AddSnort(256)
	if err != nil {
		panic(err)
	}
	defer vc.Stop(snort)
	fmt.Println("started snort:", snort)

	client, err := vc.AddClient(server, 1024)
	if err != nil {
		panic(err)
	}
	defer vc.Stop(client)
	fmt.Println("started client:", client)

	err = vc.Route(client, snort, server)
	if err != nil {
		panic(err)
	} else {
		fmt.Println("route setup")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	timeout := time.After(10 * 60 * time.Second)
	select {
	case <-sigs:
	case <-timeout:
	}
	fmt.Println("done with the experiment")
}
