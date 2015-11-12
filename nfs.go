package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Unknwon/goconfig"
	"github.com/VividCortex/godaemon"
	"github.com/mangalaman93/nfs/nfsmain"
)

func parseArgs(daemonize *bool, cfile *string) {
	flag.BoolVar(daemonize, "d", false, "daemonize nfs")
	flag.StringVar(cfile, "c", ".voip.conf", "abs path to configuration file")
	flag.Parse()
}

func main() {
	var logfile *os.File
	var err error
	var cfile string

	// we open files before daemonizing the process so that
	// no error occurrs in creating log file in child process
	daemonize := true
	if godaemon.Stage() == godaemon.StageParent {
		parseArgs(&daemonize, &cfile)

		if daemonize {
			config, err := goconfig.LoadConfigFile(cfile)
			if err != nil {
				fmt.Println("[ERROR] error in reading config file:", err)
				panic(err)
			}

			filename, err := config.GetValue(goconfig.DEFAULT_SECTION, "log_file")
			if err != nil {
				fmt.Println("[ERROR] error in finding log_file in config:", err)
				panic(err)
			}

			logfile, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				fmt.Println("[ERROR] error opening log file:", err)
				panic(err)
			}

			err = syscall.Flock(int(logfile.Fd()), syscall.LOCK_EX)
			if err != nil {
				fmt.Println("[ERROR] error acquiring lock to log file:", err)
				panic(err)
			}
		}
	}

	if daemonize {
		_, _, err = godaemon.MakeDaemon(&godaemon.DaemonAttr{Files: []**os.File{&logfile}})
		if err != nil {
			fmt.Println("[ERROR] error daemonizing:", err)
			panic(err)
		}

		defer logfile.Close()
		log.SetOutput(logfile)
		parseArgs(&daemonize, &cfile)
	}

	log.SetFlags(log.LstdFlags)
	log.Println("#################### BEGIN OF LOG ##########################")

	// register ctrl+c
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	log.Println("[_INFO] adding signal handler for SIGTERM")

	// read configuration file
	config, err := goconfig.LoadConfigFile(cfile)
	if err != nil {
		log.Println("[ERROR] error in reading config file:", err)
		panic(err)
	}

	// control loop
	if err := nfsmain.Start(config); err != nil {
		log.Println("[ERROR] error in nfsmain loop:", err)
		panic(err)
	} else {
		defer nfsmain.Stop()
	}

	// wait for ctrl+c
	log.Println("[_INFO] waiting for ctrl+c signal")
	<-sigs

	// exit
	log.Println("[_INFO] exiting main!")
}
