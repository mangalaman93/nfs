package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"syscall"
)

import (
	"github.com/VividCortex/godaemon"
	influxdb "github.com/influxdb/influxdb/client"
)

const (
	LOG_FILE = "nfs.log"
)

func query(con *influxdb.Client, database, cmd string) (res []influxdb.Result, err error) {
	q := influxdb.Query{
		Command:  cmd,
		Database: database,
	}
	if response, err := con.Query(q); err == nil {
		if response.Error() != nil {
			return res, response.Error()
		}
		res = response.Results
	}
	return
}

func parseArgs(daemonize *bool, host *string, user *string, pass *string, database *string) {
	flag.BoolVar(daemonize, "d", false, "daemonize")
	flag.StringVar(host, "host", "localhost:8086", "<ip:port>")
	flag.StringVar(user, "username", "root", "username")
	flag.StringVar(pass, "password", "root", "password")
	flag.StringVar(database, "database", "", "database")
	flag.Parse()

	_, err := url.Parse(fmt.Sprintf("http://%s", *host))
	if err != nil {
		fmt.Println("Unable to parse ", host)
		os.Exit(1)
	}

	if *database == "" {
		fmt.Println("database required!")
		flag.Usage()
		os.Exit(1)
	}
}

func main() {
	var logfile *os.File
	var err error
	var host, user, pass, database string

	daemonize := true
	if godaemon.Stage() == godaemon.StageParent {
		parseArgs(&daemonize, &host, &user, &pass, &database)

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
		parseArgs(&daemonize, &host, &user, &pass, &database)
	}

	log.SetFlags(log.LstdFlags)
	log.Println("#################### BEGIN OF LOG ##########################")

	url, _ := url.Parse(fmt.Sprintf("http://%s", host))
	client, err := influxdb.NewClient(influxdb.Config{
		URL:      *url,
		Username: user,
		Password: pass,
	})
	if err != nil {
		log.Fatalln("[ERROR] unable to create influxdb client, ", err)
	}

	// list measurements
	log.Println(query(client, database, "SHOW MEASUREMENTS"))
}
