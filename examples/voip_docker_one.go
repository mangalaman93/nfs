package main

import (
	"fmt"
	"time"

	"github.com/mangalaman93/nfs/client"
)

func main() {
	vc, err := client.NewVoipClient("/home/ubuntu/nfs/.voip.conf")
	if err != nil {
		panic(err)
	}
	defer vc.Close()

	server, err := vc.AddServer()
	if err != nil {
		panic(err)
	}
	defer vc.Stop(server)
	fmt.Println("started server:", server)

	snort, err := vc.AddSnort()
	if err != nil {
		panic(err)
	}
	defer vc.Stop(snort)
	fmt.Println("started snort:", snort)

	client, err := vc.AddClient(server)
	if err != nil {
		panic(err)
	}
	defer vc.Stop(client)
	fmt.Println("started client:", client)

	vc.Route(client, snort, server)
	fmt.Println("route setup")
	time.Sleep(60 * time.Second)
	fmt.Println("done with the experiment")
}
