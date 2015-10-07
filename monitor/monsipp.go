package main

import (
	"flag"
	"fmt"
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
	"github.com/VividCortex/godaemon"
	dockerclient "github.com/fsouza/go-dockerclient"
	influxdb "github.com/influxdb/influxdb/client"
	"github.com/mangalaman93/tail"
)

const (
	LOG_FILE            = "sipp.log"
	CHECK_INTERVAL      = 4000
	IMAGE_SIPP          = "mangalaman93/sipp"
	PATH_VOL_SIPP       = "/data"
	INFLUXDB_DB         = "cadvisor"
	STAT_FIELD_COUNT    = 87
	MAX_LINE_LENGTH     = 3000
	NET_BUFFER_SIZE     = 1000
	INFLUXDB_BATCH_SIZE = 50
)

var MEASUREMENTS = map[int]string{
	6:  "call_rate",
	8:  "incoming_call",
	10: "outgoing_call",
	13: "current_calls",
	14: "successful_calls",
	16: "failed_calls",
	20: "failed_max_udp_retrans",
	38: "failed_outbound_congestion",
	40: "failed_timeout_on_recv",
	42: "failed_timeout_on_send",
	48: "retransmissions",
	56: "watchdog_major",
	58: "watchdog_minor",
}

type Volume struct {
	id       string
	cont     string
	rtt      *tail.Tail
	stat     *tail.Tail
	stopchan chan bool
	waitchan chan bool
}

var machine_name string
var clients []*influxdb.Client
var sippvols = map[string]*Volume{}

func (v *Volume) String() string {
	return fmt.Sprintf("{id:%s, cont:%s, rtt:%s, stat:%s}", v.id, v.cont, v.rtt, v.stat)
}

func (v *Volume) dispatchRtts(influxchan chan *influxdb.Point) {
	// skipping first line
	for _ = range v.rtt.Lines {
		break
	}

	ticker := time.NewTicker(1 * time.Second).C
	sum := 0.0
	count := 0
	for line := range v.rtt.Lines {
		select {
		case <-ticker:
			if count != 0 {
				influxchan <- &influxdb.Point{
					Measurement: "response_time",
					Tags: map[string]string{
						"container_name": v.cont,
					},
					Fields: map[string]interface{}{
						"value": sum / float64(count),
					},
					Time:      time.Now(),
					Precision: "s",
				}
				count = 0
				sum = 0
			}
		default:
			// do nothing
		}

		fields := strings.Split(line, ";")
		if len(fields) < 3 {
			log.Println("[WARN] unable to parse string:", line)
			continue
		}

		value, err := strconv.ParseFloat(fields[1], 32)
		if err != nil {
			log.Println("[WARN] unable to parse value ", fields[1])
			continue
		}

		sum += value
		count++
	}

	log.Printf("[INFO] exiting dispatchRtts for container %s with error: %s\n", v.cont, v.rtt.Err)
}

func (v *Volume) dispatchStats(influxchan chan *influxdb.Point) {
	for _ = range v.stat.Lines {
		break
	}

	for line := range v.stat.Lines {
		fields := strings.Split(line, ";")
		if len(fields) < STAT_FIELD_COUNT {
			log.Printf("[WARN] only %v fields in %s\n", len(fields), fields)
			continue
		}

		for index, measurement := range MEASUREMENTS {
			value, err := strconv.ParseFloat(fields[index], 32)
			if err != nil {
				log.Println("[WARN] unable to parse value ", fields[index])
				value = 0
			}

			influxchan <- &influxdb.Point{
				Measurement: measurement,
				Tags: map[string]string{
					"container_name": v.cont,
				},
				Fields: map[string]interface{}{
					"value": value,
				},
				Time:      time.Now(),
				Precision: "s",
			}
		}
	}

	log.Printf("[INFO] exiting dispatchStats for container %s with error: %s\n", v.cont, v.stat.Err)
}

func (v *Volume) Tail(influxchan chan *influxdb.Point) {
	var files []string
	var err error

	for {
		files, err = filepath.Glob(filepath.Join(v.id, "*.csv"))
		if err != nil {
			log.Printf("[WARN] unable to find files to read from volume %s of container %s\n", v.id, v.cont)
			continue
		}

		if len(files) == 2 {
			break
		} else if len(files) > 2 {
			log.Printf("[WARN] more than 2 .csv files present for volume %s of container %s\n", v.id, v.cont)
			return
		} else {
			log.Printf("[WARN] less than 2 .csv files present for volume %s of container %s\n", v.id, v.cont)
		}

		timeout := time.After(CHECK_INTERVAL * time.Millisecond)
		select {
		case <-v.stopchan:
			log.Printf("[INFO] no clean up required for volume %s of container %s\n", v.id, v.cont)
			v.waitchan <- true
			return
		case <-timeout:
			continue
		}
	}

	v.stat, err = tail.TailFile(files[0], MAX_LINE_LENGTH)
	if err != nil {
		log.Printf("[WARN] unable to read stat file for volume %s of container %s\n", v.id, v.cont)
	} else {
		go v.dispatchStats(influxchan)
	}

	v.rtt, err = tail.TailFile(files[1], MAX_LINE_LENGTH)
	if err != nil {
		log.Printf("[WARN] unable to read rtt file for volume %s of container %s\n", v.id, v.cont)
	} else {
		go v.dispatchRtts(influxchan)
	}

	<-v.stopchan
	v.rtt.Stop()
	v.stat.Stop()
	log.Printf("[INFO] cleaned up for volume %s of container %s\n", v.id, v.cont)
	v.waitchan <- true
}

func (v *Volume) StopTail() {
	v.stopchan <- true
	<-v.waitchan
}

func cleanupVolumes() {
	for _, v := range sippvols {
		v.StopTail()
	}

	log.Println("[INFO] cleanup done, exiting!")
}

func sendBatch(pts []influxdb.Point) {
	for _, c := range clients {
		c.Write(influxdb.BatchPoints{
			Points:          pts,
			Database:        INFLUXDB_DB,
			RetentionPolicy: "default",
			Tags:            map[string]string{"machine": machine_name},
		})
	}
}

func bgWrite(influxchan chan *influxdb.Point, done chan bool) {
	count := 0
	pts := make([]influxdb.Point, INFLUXDB_BATCH_SIZE)

	for {
		point := <-influxchan
		if point == nil {
			sendBatch(pts[:count])
			done <- true
			break
		} else {
			pts[count] = *point
			count += 1
		}

		// if buffer is full
		if count == INFLUXDB_BATCH_SIZE {
			sendBatch(pts)
			count = 0
		}
	}
}

func dockerListener(docker *dockerclient.Client, dokchan chan *dockerclient.APIEvents, done chan bool) {
	log.Println("[INFO] listening to docker container events")
	defer func() { done <- true }()

	influxchan := make(chan *influxdb.Point, NET_BUFFER_SIZE)
	donechan := make(chan bool, 1)
	go bgWrite(influxchan, donechan)
	defer func() {
		influxchan <- nil
		<-donechan
	}()

	defer func() { cleanupVolumes() }()
	for {
		event := <-dokchan
		log.Println("[INFO] event occurred:", event)

		switch event.Status {
		case "EOF":
			return

		case "start":
			if _, ok := sippvols[event.ID]; ok {
				log.Println("[WARN] duplicate event for container ", event.ID)
				continue
			}

			if event.From == IMAGE_SIPP {
				cont, err := docker.InspectContainer(event.ID)
				if err != nil {
					log.Println("[WARN] unable to inspect container ", event.ID)
					continue
				}

				volume := cont.Volumes[PATH_VOL_SIPP]
				if volume == "" {
					for _, v := range cont.Mounts {
						if v.Destination == PATH_VOL_SIPP {
							volume = v.Source
							break
						}
					}
				}

				if volume == "" {
					log.Println("[WARN] unable to find volume for container ", event.ID)
					continue
				}

				v := &Volume{
					id:       volume,
					cont:     cont.Name[1:],
					stopchan: make(chan bool, 1),
					waitchan: make(chan bool, 1),
				}

				sippvols[event.ID] = v
				go v.Tail(influxchan)
				log.Printf("[INFO] add volume %s to map for container %s\n", volume, event.ID)
			}

		case "die", "kill", "stop":
			if v, ok := sippvols[event.ID]; ok {
				v.StopTail()
				delete(sippvols, event.ID)
				log.Printf("[INFO] delete volume %s from map for container %s\n", v.id, event.ID)
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

	// check root
	if os.Geteuid() != 0 {
		fmt.Println("please run as root!")
		os.Exit(1)
	}

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

	// getting machine name
	id, err := os.Hostname()
	if err != nil {
		log.Fatalln("[ERROR] unable to get system id:", err)
	}
	machine_name = strings.TrimSpace(string(id))
	log.Println("[INFO] machine name:", machine_name)

	// influxdb clients
	args := flag.Args()
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
		log.Println("[INFO] adding influxdb client:", arg)
	}

	if index == 0 {
		log.Fatalln("[ERROR] no client is parsable, exiting!")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	log.Println("[INFO] adding signal handler for SIGTERM")

	docker, err := dockerclient.NewClientFromEnv()
	if err != nil {
		log.Fatalln("[ERROR] unable to create docker client, exiting!")
	}

	dokchan := make(chan *dockerclient.APIEvents, 100)
	waitchan := make(chan bool, 1)
	err = docker.AddEventListener(dokchan)
	if err != nil {
		log.Fatalln("[ERROR] unable to add docker event listener, exiting!")
	}
	go dockerListener(docker, dokchan, waitchan)

	// wait for stop signal and then cleanup
	_ = <-sigs
	log.Println("[INFO] beginning cleanup!")
	docker.RemoveEventListener(dokchan)
	dokchan <- dockerclient.EOFEvent
	<-waitchan
}
