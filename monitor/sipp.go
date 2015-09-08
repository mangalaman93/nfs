package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

import (
	"github.com/ActiveState/tail"
	"github.com/VividCortex/godaemon"
	dockerclient "github.com/fsouza/go-dockerclient"
	influxdb "github.com/influxdb/influxdb/client"
)

const (
	LOG_FILE        = "sipp.log"
	UPDATE_INTERVAL = 4000
	IMAGE_SIPP      = "mangalaman93/sipp"
	PATH_VOL_SIPP   = "/data"
)

type Tails struct {
	Dir      string
	Rtt      *tail.Tail
	Stat     *tail.Tail
	stopchan chan bool
	waitchan chan bool
}

var clients []*influxdb.Client
var sippvols = map[string]*Tails{}

func (t *Tails) String() string {
	return fmt.Sprintf("{Dir:%s, Rtt:%s, Stat:%s}", t.Dir, t.Rtt, t.Stat)
}

func (t *Tails) TailVolume() {
	var files []string
	var err error

	for {
		files, err = filepath.Glob(filepath.Join(t.Dir, "*.csv"))
		if err != nil {
			log.Println("[WARN] unable to find files to read from volume ", t.Dir)
			continue
		}

		if len(files) == 2 {
			break
		} else if len(files) > 2 {
			log.Println("[WARN] more than 2 .csv files present for volume ", t.Dir)
			return
		} else {
			log.Println("[WARN] less than 2 .csv files present for volume ", t.Dir)
		}

		timeout := time.After(UPDATE_INTERVAL * time.Millisecond)
		select {
		case <-t.stopchan:
			log.Println("[INFO] no clean up required for volume ", t.Dir)
			t.waitchan <- true
			return
		case <-timeout:
			continue
		}
	}

	t.Stat, err = tail.TailFile(files[0], tail.Config{Follow: true, ReOpen: false})
	if err != nil {
		log.Println("[WARN] unable to read stat file for volume ", t.Dir)
	} else {
		go dispatchStats(t.Stat)
	}

	t.Rtt, err = tail.TailFile(files[1], tail.Config{Follow: true, ReOpen: false})
	if err != nil {
		log.Println("[WARN] unable to read rtt file for volume ", t.Dir)
	} else {
		go dispatchRtts(t.Rtt)
	}

	<-t.stopchan
	t.cleanTail()
}

func (t *Tails) StopTail() {
	t.stopchan <- true
	<-t.waitchan
}

func (t *Tails) cleanTail() {
	t.Rtt.Stop()
	t.Stat.Stop()
	t.Rtt.Cleanup()
	t.Stat.Cleanup()

	log.Println("[INFO] cleaned up for volume ", t.Dir)
	t.waitchan <- true
}

func dispatchRtts(t *tail.Tail) {
	for line := range t.Lines {
		fmt.Println(line.Text)
	}
}

func dispatchStats(t *tail.Tail) {
	for line := range t.Lines {
		fmt.Println(line.Text)
	}
}

func cleanup() {
	for _, tails := range sippvols {
		tails.StopTail()
	}

	log.Println("[INFO] cleanup done, exiting!")
}

func dockerListener(docker *dockerclient.Client, chand chan *dockerclient.APIEvents) {
	log.Println("[INFO] listening to docker container events")

	for {
		event := <-chand
		log.Println("[INFO] event occurred: ", event)

		switch event.Status {
		case "EOF":
			break

		case "start":
			if _, ok := sippvols[event.ID]; ok {
				continue
			}

			if event.From == IMAGE_SIPP {
				cont, err := docker.InspectContainer(event.ID)
				if err != nil {
					log.Println("[WARN] unable to inspect container ", event.ID)
					continue
				}

				volume := cont.Volumes[PATH_VOL_SIPP]
				tails := &Tails{Dir: volume, stopchan: make(chan bool, 1), waitchan: make(chan bool, 1)}
				sippvols[event.ID] = tails
				go tails.TailVolume()
				log.Printf("[INFO] add volume %s to map for container %s\n", volume, event.ID)
			}

		case "die", "kill", "stop":
			if tails, ok := sippvols[event.ID]; ok {
				go tails.StopTail()
				delete(sippvols, event.ID)
				log.Printf("[INFO] delete volume %s from map for container %s\n", tails.Dir, event.ID)
			}
		}
	}
}

func parseArgs(daemonize *bool) {
	flag.BoolVar(daemonize, "d", false, "daemonize")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Printf("[ERROR] %s requires more arguments!\n", os.Args[0])
		os.Exit(1)
	}
}

func main() {
	var logfile *os.File
	var err error
	var args []string
	daemonize := true

	// check root
	if os.Geteuid() != 0 {
		fmt.Println("please run as root!")
		os.Exit(1)
	}

	// parent
	if godaemon.Stage() == godaemon.StageParent {
		// command line flags
		parseArgs(&daemonize)
		args = flag.Args()

		if daemonize {
			// log settings
			logfile, err = os.OpenFile(LOG_FILE, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				fmt.Printf("[ERROR] error opening log file: %v", err)
				os.Exit(1)
			}

			err = syscall.Flock(int(logfile.Fd()), syscall.LOCK_EX)
			if err != nil {
				fmt.Printf("[ERROR] error acquiring lock to log file: %v", err)
				os.Exit(1)
			}
		}
	}

	// daemonize
	if daemonize {
		_, _, err = godaemon.MakeDaemon(&godaemon.DaemonAttr{
			Files: []**os.File{&logfile},
		})
		if err != nil {
			fmt.Printf("[ERROR] error daemonizing: %v", err)
			os.Exit(1)
		}

		defer logfile.Close()
		log.SetOutput(logfile)
		parseArgs(&daemonize)
		args = flag.Args()
	}

	log.SetFlags(log.LstdFlags)
	log.Println("#################### BEGIN OF LOG ##########################")

	// influxdb clients
	clients = make([]*influxdb.Client, len(args))
	index := 0
	for _, arg := range args {
		host, err := url.Parse(fmt.Sprintf("http://%s", arg))
		if err != nil {
			log.Printf("[WARN] unable to parse %s!\n", arg)
			continue
		}

		client, err := influxdb.NewClient(influxdb.Config{URL: *host})
		if err != nil {
			log.Printf("[WARN] unable to create influxdb client for %s!\n", arg)
			continue
		}

		clients[index] = client
		index += 1
		log.Println("[INFO] adding influxdb client: ", arg)
	}

	if index == 0 {
		log.Fatalln("[ERROR] no client is parsable, exiting!")
	}

	// sigterm handler
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	log.Println("[INFO] adding signal handler for SIGTERM")

	// start docker event monitoring
	docker, err := dockerclient.NewClientFromEnv()
	if err != nil {
		log.Fatalln("[ERROR] unable to create docker client, exiting!")
	}

	chand := make(chan *dockerclient.APIEvents, 100)
	err = docker.AddEventListener(chand)
	if err != nil {
		log.Fatalln("[ERROR] unable to add docker event listener, exiting!")
	}
	go dockerListener(docker, chand)

	// wait for stop signal and then cleanup
	_ = <-sigs
	log.Println("[INFO] beginning cleanup!")

	// cleanup
	docker.RemoveEventListener(chand)
	chand <- dockerclient.EOFEvent
	cleanup()
}
