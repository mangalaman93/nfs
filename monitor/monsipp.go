package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	FILE_READ_PERIOD    = 500
	SIPP_UDP_PORT       = 5060
	IMAGE_SNORT         = "mangalaman93/snort"
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
	wg       sync.WaitGroup
}

type Snort struct {
	pid      int
	cont     string
	stopchan chan bool
	waitchan chan bool
}

var machine_name string
var clients []*influxdb.Client
var sippvols = map[string]*Volume{}
var snorts = map[string]*Snort{}

func (s *Snort) String() string {
	return fmt.Sprintf("{pid:%s, cont:%s}", s.pid, s.cont)
}

func (s *Snort) StopTail() {
	close(s.stopchan)
	<-s.waitchan
}

func (s *Snort) TailQueue(influxchan chan *influxdb.Point) {
	// wait for queue to be created
	time.Sleep(2000 * time.Millisecond)

	// check whether the file exists
	netfilter_file := path.Join("/proc/", strconv.Itoa(s.pid), "/net/netfilter/nfnetlink_queue")
	if _, err := os.Stat(netfilter_file); err != nil {
		log.Printf("[ERROR] %s file not found!\n", netfilter_file)
		panic(err)
	}

	// check whether the dev file exists
	dev_file := path.Join("/proc/", strconv.Itoa(s.pid), "/net/dev")
	if _, err := os.Stat(dev_file); err != nil {
		log.Printf("[ERROR] %s file not found!\n", dev_file)
		panic(err)
	}

	defer close(s.waitchan)
	for {
		select {
		case <-s.stopchan:
			log.Println("[INFO] exiting tailQueue for container", s.cont)
			return
		default:
			time.Sleep(time.Duration(FILE_READ_PERIOD) * time.Millisecond)
		}

		out, err := ioutil.ReadFile(netfilter_file)
		curtime := time.Now()
		if err != nil {
			log.Println("[WARN] unable to read netfilter file:", err)
			return
		}

		row := strings.Fields(string(out))
		if len(row) < 9 {
			log.Println("[WARN] incorrect parsing of netfilter file, parsed line:", row)
			continue
		}

		drops, err := strconv.ParseInt(row[5], 10, 64)
		if err != nil {
			log.Println("[WARN] unable to convert number of queue drops,", err)
		} else {
			influxchan <- &influxdb.Point{
				Measurement: "snort_queue_drops",
				Tags: map[string]string{
					"container_name": s.cont,
				},
				Fields: map[string]interface{}{
					"value": drops,
				},
				Time: curtime,
			}
		}

		drops, err = strconv.ParseInt(row[6], 10, 64)
		if err != nil {
			log.Println("[WARN] unable to convert number of user drops,", err)
		} else {
			influxchan <- &influxdb.Point{
				Measurement: "snort_user_drops",
				Tags: map[string]string{
					"container_name": s.cont,
				},
				Fields: map[string]interface{}{
					"value": drops,
				},
				Time: curtime,
			}
		}

		// dev file
		out, err = ioutil.ReadFile(dev_file)
		curtime = time.Now()
		if err != nil {
			log.Println("[WARN] unable to read dev file:", err)
			return
		}

		rows := strings.Split(string(out), "\n")
		if len(rows) < 4 {
			log.Println("[WARN] incorrect number of lines in dev file for container", s.cont)
			continue
		}

		index := 3
		if strings.Contains(rows[2], "lo") {
			if strings.Contains(rows[3], "lo") {
				log.Println("[WARN] unable to find correct network interface for container", s.cont)
				continue
			}
		} else {
			index = 2
		}

		row = strings.Fields(rows[index])
		if len(row) < 17 {
			log.Println("[WARN] incorrect parsing of dev file, parsed line:", row)
			continue
		}

		rx_packets, err := strconv.ParseInt(row[2], 10, 64)
		if err != nil {
			log.Println("[WARN] unable to convert number of received packets,", err)
		} else {
			influxchan <- &influxdb.Point{
				Measurement: "rx_packets",
				Tags: map[string]string{
					"container_name": s.cont,
				},
				Fields: map[string]interface{}{
					"value": rx_packets,
				},
				Time: curtime,
			}
		}

		tx_packets, err := strconv.ParseInt(row[10], 10, 64)
		if err != nil {
			log.Println("[WARN] unable to convert number of tx packets,", err)
		} else {
			influxchan <- &influxdb.Point{
				Measurement: "tx_packets",
				Tags: map[string]string{
					"container_name": s.cont,
				},
				Fields: map[string]interface{}{
					"value": tx_packets,
				},
				Time: curtime,
			}
		}
	}

	log.Println("[INFO] exiting tailQueue for container", s.cont)
}

func (v *Volume) String() string {
	return fmt.Sprintf("{id:%s, cont:%s, rtt:%s, stat:%s}", v.id, v.cont, v.rtt, v.stat)
}

func (v *Volume) dispatchRtts(influxchan chan *influxdb.Point) {
	v.wg.Add(1)
	defer v.wg.Done()

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
					Time: time.Now(),
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
			log.Println("[WARN] unable to parse value", fields[1])
			continue
		}

		sum += value
		count++
	}

	log.Printf("[INFO] exiting dispatchRtts for container %s with error: %s\n", v.cont, v.rtt.Err)
}

func (v *Volume) dispatchStats(influxchan chan *influxdb.Point) {
	v.wg.Add(1)
	defer v.wg.Done()

	for _ = range v.stat.Lines {
		break
	}

	for line := range v.stat.Lines {
		fields := strings.Split(line, ";")
		if len(fields) < STAT_FIELD_COUNT {
			log.Printf("[WARN] only %v fields in %s\n", len(fields), fields)
			continue
		}

		curtime := time.Now()
		for index, measurement := range MEASUREMENTS {
			value, err := strconv.ParseFloat(fields[index], 32)
			if err != nil {
				log.Println("[WARN] unable to parse value", fields[index])
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
				Time: curtime,
			}
		}
	}

	log.Printf("[INFO] exiting dispatchStats for container %s with error: %s\n", v.cont, v.stat.Err)
}

func (v *Volume) tailUDP(influxchan chan *influxdb.Point, pid int) {
	v.wg.Add(1)
	defer v.wg.Done()

	hex_port := strings.ToUpper(fmt.Sprintf("%x", SIPP_UDP_PORT))
	udp_file := path.Join("/proc/", strconv.Itoa(pid), "/net/udp")
	if _, err := os.Stat(udp_file); err != nil {
		log.Printf("[ERROR] %s file not found!\n", udp_file)
		panic(err)
	}

	for {
		select {
		case <-v.stopchan:
			log.Println("[INFO] exiting tailUDP for container", v.cont)
			return
		default:
			time.Sleep(time.Duration(FILE_READ_PERIOD) * time.Millisecond)
		}

		out, err := ioutil.ReadFile(udp_file)
		curtime := time.Now()
		if err != nil {
			log.Println("[WARN] unable to read udp file:", err)
			return
		}

		line := ""
		flag := true
		for _, line = range strings.Split(string(out), "\n") {
			if strings.Contains(line, hex_port) {
				flag = false
				break
			}
		}

		if flag {
			log.Println("[WARN] unable to find udp port for container", v.cont)
			continue
		}

		row := strings.Fields(line)
		if len(row) < 13 {
			log.Println("[WARN] incorrect parsing of udp file, parsed line:", row)
			continue
		}

		if strings.Index(row[4], ":") == -1 {
			log.Println("[WARN] unable to parse tx/rx queues:", row[4])
			continue
		}
		rx_tx := strings.Split(row[4], ":")

		txqueue, err := strconv.ParseInt(rx_tx[0], 16, 64)
		if err != nil {
			log.Println("[WARN] unable to convert tx queue size,", err)
		} else {
			influxchan <- &influxdb.Point{
				Measurement: "txqueue_udp",
				Tags: map[string]string{
					"container_name": v.cont,
				},
				Fields: map[string]interface{}{
					"value": txqueue,
				},
				Time: curtime,
			}
		}

		rxqueue, err := strconv.ParseInt(rx_tx[1], 16, 64)
		if err != nil {
			log.Println("[WARN] unable to convert rx queue size,", err)
		} else {
			influxchan <- &influxdb.Point{
				Measurement: "rxqueue_udp",
				Tags: map[string]string{
					"container_name": v.cont,
				},
				Fields: map[string]interface{}{
					"value": rxqueue,
				},
				Time: curtime,
			}
		}

		drops, err := strconv.ParseInt(row[12], 10, 64)
		if err != nil {
			log.Println("[WARN] unable to convert number of drops,", err)
		} else {
			influxchan <- &influxdb.Point{
				Measurement: "drops_udp",
				Tags: map[string]string{
					"container_name": v.cont,
				},
				Fields: map[string]interface{}{
					"value": drops,
				},
				Time: curtime,
			}
		}
	}
}

func (v *Volume) Tail(influxchan chan *influxdb.Point, pid int) {
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
			close(v.waitchan)
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

	// go routine to collect UDP queue size
	go v.tailUDP(influxchan, pid)

	// wait for stop signal
	<-v.stopchan
	v.rtt.Stop()
	v.stat.Stop()
	v.wg.Wait()
	log.Printf("[INFO] cleaned up for volume %s of container %s\n", v.id, v.cont)
	close(v.waitchan)
}

func (v *Volume) StopTail() {
	close(v.stopchan)
	<-v.waitchan
}

func cleanupVolumes() {
	for _, v := range sippvols {
		v.StopTail()
	}

	for _, s := range snorts {
		s.StopTail()
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
		point, more := <-influxchan
		if more {
			pts[count] = *point
			count += 1
		} else {
			sendBatch(pts[:count])
			break
		}

		// if buffer is full
		if count == INFLUXDB_BATCH_SIZE {
			sendBatch(pts)
			count = 0
		}
	}

	log.Println("[INFO] exiting influxdb routine!")
	close(done)
}

func dockerListener(docker *dockerclient.Client, dokchan chan *dockerclient.APIEvents, done chan bool) {
	log.Println("[INFO] listening to docker container events")
	defer func() {
		log.Println("[INFO] exiting docker listener!")
		close(done)
	}()

	influxchan := make(chan *influxdb.Point, NET_BUFFER_SIZE)
	donechan := make(chan bool)
	go bgWrite(influxchan, donechan)
	defer func() {
		close(influxchan)
		<-donechan
	}()

	defer func() { cleanupVolumes() }()
	for {
		event, more := <-dokchan
		if !more {
			return
		}

		log.Println("[INFO] event occurred:", event)
		switch event.Status {
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
					stopchan: make(chan bool),
					waitchan: make(chan bool),
				}

				sippvols[event.ID] = v
				go v.Tail(influxchan, cont.State.Pid)
				log.Printf("[INFO] add volume %s to map for container %s\n", volume, event.ID)
			} else if event.From == IMAGE_SNORT {
				cont, err := docker.InspectContainer(event.ID)
				if err != nil {
					log.Println("[WARN] unable to inspect container ", event.ID)
					continue
				}

				s := &Snort{
					pid:      cont.State.Pid,
					cont:     cont.Name[1:],
					stopchan: make(chan bool),
					waitchan: make(chan bool),
				}

				snorts[event.ID] = s
				go s.TailQueue(influxchan)
				log.Printf("[INFO] add into tail list container %s\n", event.ID)
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
	waitchan := make(chan bool)
	err = docker.AddEventListener(dokchan)
	if err != nil {
		log.Fatalln("[ERROR] unable to add docker event listener, exiting!")
	}
	go dockerListener(docker, dokchan, waitchan)

	// wait for stop signal and then cleanup
	_ = <-sigs
	log.Println("[INFO] beginning cleanup!")
	docker.RemoveEventListener(dokchan)
	close(dokchan)
	<-waitchan
	log.Println("[INFO] goodbye!")
}
