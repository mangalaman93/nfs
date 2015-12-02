package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/VividCortex/godaemon"
)

const (
	LOG_FILE = "contmon.log"
)

func parseArgs(daemonize *bool) {
	flag.BoolVar(daemonize, "d", false, "daemonize")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Printf("[ERROR] %s requires more arguments!\n", os.Args[0])
		os.Exit(1)
	}
}

func main() {
	if os.Geteuid() != 0 {
		fmt.Println("please run as root!")
		os.Exit(1)
	}

	var logfile *os.File
	var err error
	daemonize := true
	if godaemon.Stage() == godaemon.StageParent {
		parseArgs(&daemonize)
		if daemonize {
			logfile, err = os.OpenFile(LOG_FILE, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				fmt.Println("[ERROR] error opening log file:", err)
				os.Exit(1)
			}

			err = syscall.Flock(int(logfile.Fd()), syscall.LOCK_EX)
			if err != nil {
				fmt.Println("[ERROR] error acquiring lock to log file:", err)
				os.Exit(1)
			}
		}
	}

	if daemonize {
		_, _, err = godaemon.MakeDaemon(&godaemon.DaemonAttr{Files: []**os.File{&logfile}})
		if err != nil {
			fmt.Println("[ERROR] error daemonizing:", err)
			os.Exit(1)
		}

		defer logfile.Close()
		log.SetOutput(logfile)
		parseArgs(&daemonize)
	}
	log.SetFlags(log.LstdFlags)
	log.Println("#################### BEGIN OF LOG ##########################")

	InitConfig()
	dbclient, err := NewDBClient(flag.Args())
	if err != nil {
		log.Println("[ERROR] unable to create influxdb client", err)
		panic(err)
	}
	err = dbclient.Start()
	if err != nil {
		log.Println("[ERROR] unable to run influxdb client", err)
		panic(err)
	}
	defer dbclient.Stop()

	handler, err := NewDockerHandler(dbclient)
	if err != nil {
		log.Println("[ERROR] unable to create docker handler", err)
		panic(err)
	}
	err = handler.Start()
	if err != nil {
		log.Println("[ERROR] unable to run docker handler", err)
		panic(err)
	}
	defer handler.Stop()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	log.Println("[INFO] goodbye!")
}
