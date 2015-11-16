package main

import (
	"fmt"
	"time"

	"github.com/mangalaman93/nfs/client"
)

func main() {
	vc, err := client.NewVoipClient("/home/ubuntu/nfs/.voip.vonf")
	if err != nil {
		panic(err)
	}

	server, err := vc.AddServer()
	defer vc.Stop(server)
	if err != nil {
		panic(err)
	}
	fmt.Println("started server:", server)

	snort, err := vc.AddSnort()
	defer vc.Stop(snort)
	if err != nil {
		panic(err)
	}
	fmt.Println("started snort:", snort)

	client, err := vc.AddClient(server)
	defer vc.Stop(client)
	if err != nil {
		panic(err)
	}
	fmt.Println("started client:", client)

	vc.Route(client, snort, server)
	fmt.Println("route setup")

	time.Sleep(60 * time.Second)
	vc.Close()
	fmt.Println("done with the experiment")
}
