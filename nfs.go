package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/VividCortex/godaemon"
	"github.com/mangalaman93/nfs/nfsmain"
	"github.com/mangalaman93/nfs/voip"
)

const (
	LOG_FILE  = "nfs.log"
	UNIX_SOCK = "nfs.sock"
)

func parseArgs(daemonize *bool, port *string) {
	flag.BoolVar(daemonize, "d", false, "daemonize nfs")
	flag.StringVar(port, "p", "8086", "port")
	flag.Parse()
}

func main() {
	var logfile *os.File
	var err error
	var port string

	daemonize := true
	if godaemon.Stage() == godaemon.StageParent {
		parseArgs(&daemonize, &port)

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
		parseArgs(&daemonize, &port)
	}

	log.SetFlags(log.LstdFlags)
	log.Println("#################### BEGIN OF LOG ##########################")

	// register ctrl+c
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	log.Println("[INFO] adding signal handler for SIGTERM")

	// control loop
	if err := nfsmain.Listen(port); err != nil {
		log.Fatalln("[ERROR] error in nfsmain loop:", err)
	}

	// voip command unix socket loop
	if err := voip.Listen(UNIX_SOCK); err != nil {
		log.Fatalln("[ERROR] error in voip loop:", err)
	}

	// wait for ctrl+c
	log.Println("[INFO] waiting for ctrl+c signal")
	<-sigs

	// clean up
	nfsmain.Stop()
	voip.Stop()
	log.Println("[INFO] clean up done, exiting!")
}
