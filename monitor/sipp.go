package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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
	INFLUXDB_DB     = "cadvisor"
	FIELD_COUNT     = 87
)

var MEASUREMENTS = map[int]string{
	6:  "call_rate",
	8:  "incoming_call",
	10: "outgoing_call",
	14: "successful_calls",
	16: "failed_calls",
	21: "failed_max_udp_retrans",
	39: "failed_outbound_congestion",
	40: "failed_timeout_on_recv",
	42: "failed_timeout_on_send",
	48: "retransmissions",
	56: "watchdog_major",
	58: "watchdog_minor",
}

type Tails struct {
	Cont     string
	Dir      string
	Rtt      *tail.Tail
	Stat     *tail.Tail
	stopchan chan bool
	waitchan chan bool
}

var machine_name string
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
			log.Printf("[WARN] unable to find files to read from volume %s of container %s\n", t.Dir, t.Cont)
			continue
		}

		if len(files) == 2 {
			break
		} else if len(files) > 2 {
			log.Printf("[WARN] more than 2 .csv files present for volume %s of container %s\n", t.Dir, t.Cont)
			return
		} else {
			log.Printf("[WARN] less than 2 .csv files present for volume %s of container %s\n", t.Dir, t.Cont)
		}

		timeout := time.After(UPDATE_INTERVAL * time.Millisecond)
		select {
		case <-t.stopchan:
			log.Printf("[INFO] no clean up required for volume %s of container %s\n", t.Dir, t.Cont)
			t.waitchan <- true
			return
		case <-timeout:
			continue
		}
	}

	t.Stat, err = tail.TailFile(files[0], tail.Config{Follow: true, ReOpen: false})
	if err != nil {
		log.Printf("[WARN] unable to read stat file for volume %s of container %s\n", t.Dir, t.Cont)
	} else {
		go dispatchStats(t.Stat, t.Cont)
	}

	t.Rtt, err = tail.TailFile(files[1], tail.Config{Follow: true, ReOpen: false})
	if err != nil {
		log.Printf("[WARN] unable to read rtt file for volume %s of container %s\n", t.Dir, t.Cont)
	} else {
		go dispatchRtts(t.Rtt, t.Cont)
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

	log.Printf("[INFO] cleaned up for volume %s of container %s\n", t.Dir, t.Cont)
	t.waitchan <- true
}

func dispatchRtts(t *tail.Tail, container_name string) {
	firstline := true
	for line := range t.Lines {
		if firstline {
			firstline = false
			continue
		}

		fields := strings.Split(line.Text, ";")
		if len(fields) < 3 {
			log.Println("[WARN] unable to parse string: ", line.Text)
			continue
		}

		value, err := strconv.ParseFloat(fields[1], 32)
		if err != nil {
			log.Println("[WARN] unable to parse value ", fields[1])
			continue
		}

		point := influxdb.Point{
			Measurement: "response_time",
			Tags: map[string]string{
				"container_name": container_name,
				"machine":        machine_name,
			},
			Fields: map[string]interface{}{
				"value": value,
			},
			Time:      time.Now(),
			Precision: "s",
		}

		for _, c := range clients {
			c.Write(influxdb.BatchPoints{
				Points:          []influxdb.Point{point},
				Database:        INFLUXDB_DB,
				RetentionPolicy: "default",
			})
		}
	}
}

func dispatchStats(t *tail.Tail, container_name string) {
	length := len(MEASUREMENTS)
	pts := make([]influxdb.Point, length)

	firstline := true
	for line := range t.Lines {
		if firstline {
			firstline = false
			continue
		}

		fields := strings.Split(line.Text, ";")
		if len(fields) < FIELD_COUNT {
			log.Printf("[WARN] only %v fields in %s\n", len(fields), fields)
			continue
		}

		count := 0
		for index, measurement := range MEASUREMENTS {
			value, err := strconv.ParseFloat(fields[index], 32)
			if err != nil {
				log.Println("[WARN] unable to parse value ", fields[index])
				value = 0
			}

			pts[count] = influxdb.Point{
				Measurement: measurement,
				Tags: map[string]string{
					"container_name": container_name,
					"machine":        machine_name,
				},
				Fields: map[string]interface{}{
					"value": value,
				},
				Time:      time.Now(),
				Precision: "s",
			}

			count += 1
		}

		for _, c := range clients {
			c.Write(influxdb.BatchPoints{
				Points:          pts,
				Database:        INFLUXDB_DB,
				RetentionPolicy: "default",
			})
		}
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
				tails := &Tails{
					Cont:     cont.Name[1:],
					Dir:      volume,
					stopchan: make(chan bool, 1),
					waitchan: make(chan bool, 1),
				}
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

	// getting machine name
	id, err := ioutil.ReadFile(filepath.Join("/sys/class/dmi/id", "product_uuid"))
	if err != nil {
		log.Fatalln("[ERROR] unable to get system id: ", err)
	}
	machine_name = strings.TrimSpace(string(id))
	log.Println("[INFO] machine name: ", machine_name)

	// influxdb clients
	clients = make([]*influxdb.Client, len(args))
	index := 0
	for _, arg := range args {
		fields := strings.Split(arg, ":")
		if len(fields) < 2 || len(fields) == 3 || len(fields) > 4 {
			log.Printf("[WARN] unable to parse %s!\n", arg)
			continue
		}

		host, err := url.Parse(fmt.Sprintf("http://%s:%s", fields[0], fields[1]))
		if err != nil {
			log.Printf("[WARN] unable to parse %s!\n", arg)
			continue
		}

		var conf influxdb.Config
		if len(fields) == 4 {
			conf = influxdb.Config{
				URL:      *host,
				Username: fields[2],
				Password: fields[3],
			}
		} else {
			conf = influxdb.Config{URL: *host}
		}

		client, err := influxdb.NewClient(conf)
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
