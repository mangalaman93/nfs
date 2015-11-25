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
	LOG_FILE             = "contmon.log"
	FILES_CHECK_INTERVAL = 4000
	SNORT_READ_PERIOD    = 500
	IMAGE_SIPP           = "mangalaman93/sipp"
	IMAGE_SNORT          = "mangalaman93/snort"
	IMAGE_SURICATA       = "mangalaman93/suricata"
	INFLUXDB_DB          = "cadvisor"
	INFLUXDB_BUFFER_SIZE = 1000
	INFLUXDB_BUFFER_DUR  = 5000
	SIPP_UDP_PORT        = 5060
	SIPP_PATH_VOL        = "/data"
	STAT_FIELD_COUNT     = 87
	MAX_LINE_LENGTH      = 3000
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

var (
	machine_name string
	clients      []*influxdb.Client
	sippvols     = map[string]*SippCont{}
	nfcs         = map[string]*NFCont{}
)

type NFCont struct {
	pid      int
	id       string
	stopchan chan bool
	waitchan chan bool
}

func (c *NFCont) String() string {
	return fmt.Sprintf("{pid:%s, id:%s}", c.pid, c.id)
}

func (c *NFCont) StopTail() {
	close(c.stopchan)
	<-c.waitchan
}

func (c *NFCont) Tail(influxchan chan *influxdb.Point) {
	defer close(c.waitchan)

	// wait for queue to be created
	netfilter_file := path.Join("/proc/", strconv.Itoa(c.pid), "/net/netfilter/nfnetlink_queue")
	dev_file := path.Join("/proc/", strconv.Itoa(c.pid), "/net/dev")
	for {
		timeout := time.After(FILES_CHECK_INTERVAL * time.Millisecond)
		select {
		case <-c.stopchan:
			log.Printf("[INFO] no clean up required for container %s\n", c.id)
			return
		case <-timeout:
		}

		if _, err := os.Stat(dev_file); err != nil {
			log.Printf("[WARN] %s file not found!\n", dev_file)
			continue
		}

		if _, err := os.Stat(netfilter_file); err != nil {
			log.Printf("[WARN] %s file not found!\n", netfilter_file)
			continue
		}

		break
	}

	for {
		select {
		case <-c.stopchan:
			log.Println("[INFO] exiting Tail for container", c.id)
			return
		default:
			time.Sleep(time.Duration(SNORT_READ_PERIOD) * time.Millisecond)
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

		parseAndSend(row[2], 10, influxchan, "snort_queue_length", c.id, curtime)
		parseAndSend(row[5], 10, influxchan, "snort_queue_drops", c.id, curtime)
		parseAndSend(row[6], 10, influxchan, "snort_user_drops", c.id, curtime)

		// dev file
		out, err = ioutil.ReadFile(dev_file)
		curtime = time.Now()
		if err != nil {
			log.Println("[WARN] unable to read dev file:", err)
			return
		}

		rows := strings.Split(string(out), "\n")
		if len(rows) < 4 {
			log.Println("[WARN] incorrect number of lines in dev file for container", c.id)
			continue
		}

		index := 3
		if strings.Contains(rows[2], "lo") {
			if strings.Contains(rows[3], "lo") {
				log.Println("[WARN] unable to find correct network interface for container", c.id)
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

		parseAndSend(row[2], 10, influxchan, "rx_packets", c.id, curtime)
		parseAndSend(row[10], 10, influxchan, "tx_packets", c.id, curtime)
	}

	log.Println("[INFO] exiting Tail for container", c.id)
}

type SippCont struct {
	id       string
	vid      string
	rtt      *tail.Tail
	stat     *tail.Tail
	stopchan chan bool
	wg       sync.WaitGroup
}

func (c *SippCont) String() string {
	return fmt.Sprintf("{id:%s, vid:%s, rtt:%s, stat:%s}", c.id, c.vid, c.rtt, c.stat)
}

func (c *SippCont) dispatchRtts(influxchan chan *influxdb.Point) {
	c.wg.Add(1)
	defer c.wg.Done()

	// skipping first line
	for _ = range c.rtt.Lines {
		break
	}

	ticker := time.NewTicker(1 * time.Second).C
	sum := 0.0
	count := 0
	for line := range c.rtt.Lines {
		select {
		case <-ticker:
			if count == 0 {
				break
			}

			influxchan <- &influxdb.Point{
				Measurement: "response_time",
				Tags: map[string]string{
					"container_name": c.id,
				},
				Fields: map[string]interface{}{
					"value": sum / float64(count),
				},
				Time: time.Now(),
			}
			count = 0
			sum = 0
		default:
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

	log.Printf("[INFO] exiting dispatchRtts for container %s with error: %s\n", c.id, c.rtt.Err)
}

func (c *SippCont) dispatchStats(influxchan chan *influxdb.Point) {
	c.wg.Add(1)
	defer c.wg.Done()

	for _ = range c.stat.Lines {
		break
	}

	for line := range c.stat.Lines {
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
					"container_name": c.id,
				},
				Fields: map[string]interface{}{
					"value": value,
				},
				Time: curtime,
			}
		}
	}

	log.Printf("[INFO] exiting dispatchStats for container %s with error: %s\n", c.id, c.stat.Err)
}

func (c *SippCont) tailUDP(influxchan chan *influxdb.Point, pid int) {
	c.wg.Add(1)
	defer c.wg.Done()

	hex_port := strings.ToUpper(fmt.Sprintf("%x", SIPP_UDP_PORT))
	udp_file := path.Join("/proc/", strconv.Itoa(pid), "/net/udp")
	if _, err := os.Stat(udp_file); err != nil {
		log.Printf("[ERROR] %s file not found!\n", udp_file)
		panic(err)
	}

	for {
		select {
		case <-c.stopchan:
			log.Println("[INFO] exiting tailUDP for container", c.id)
			return
		default:
			time.Sleep(time.Duration(SNORT_READ_PERIOD) * time.Millisecond)
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
			log.Println("[WARN] unable to find udp port for container", c.id)
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

		parseAndSend(rx_tx[0], 16, influxchan, "txqueue_udp", c.id, curtime)
		parseAndSend(rx_tx[1], 16, influxchan, "rxqueue_udp", c.id, curtime)
		parseAndSend(row[12], 10, influxchan, "drops_udp", c.id, curtime)
	}
}

func (c *SippCont) Tail(influxchan chan *influxdb.Point, pid int) {
	c.wg.Add(1)
	defer c.wg.Done()

	var files []string
	var err error
	for {
		timeout := time.After(FILES_CHECK_INTERVAL * time.Millisecond)
		select {
		case <-c.stopchan:
			log.Printf("[INFO] no clean up required for container %s\n", c.id)
			return
		case <-timeout:
		}

		files, err = filepath.Glob(filepath.Join(c.vid, "*.csv"))
		if err != nil {
			log.Printf("[WARN] unable to find files for container %s\n", c.id)
			continue
		}

		if len(files) == 2 {
			break
		} else if len(files) > 2 {
			log.Printf("[WARN] more than 2 .csv files present for container %s\n", c.id)
			return
		} else {
			log.Printf("[WARN] less than 2 .csv files present for container %s\n", c.id)
		}
	}

	c.stat, err = tail.TailFile(files[0], MAX_LINE_LENGTH)
	if err != nil {
		log.Printf("[WARN] unable to read stat file for container %s\n", c.id)
	} else {
		go c.dispatchStats(influxchan)
	}

	c.rtt, err = tail.TailFile(files[1], MAX_LINE_LENGTH)
	if err != nil {
		log.Printf("[WARN] unable to read rtt file for container %s\n", c.id)
	} else {
		go c.dispatchRtts(influxchan)
	}

	// go routine to collect UDP queue size
	go c.tailUDP(influxchan, pid)

	// wait for stop signal
	// so that in case StopTail is called even before control of the program reached here, i.e
	// if c.rtt and c.stat are not initialised, calling Stop() will result in segmentation fault
	// when the Stop() function is called in StopTail() function (before init of rtt and stat)
	<-c.stopchan
	c.rtt.Stop()
	c.stat.Stop()
}

func (c *SippCont) StopTail() {
	close(c.stopchan)
	c.wg.Wait()
	log.Printf("[INFO] cleaned up for container %s\n", c.id)
}

func parseAndSend(val string, base int, influxchan chan *influxdb.Point, table, contid string, curtime time.Time) {
	ival, err := strconv.ParseInt(val, base, 64)
	if err != nil {
		log.Println("[WARN] unable to convert parse", val, "err:", err)
		return
	}

	influxchan <- &influxdb.Point{
		Measurement: table,
		Tags: map[string]string{
			"container_name": contid,
		},
		Fields: map[string]interface{}{
			"value": ival,
		},
		Time: curtime,
	}
}

func cleanupConts() {
	for _, c := range sippvols {
		c.StopTail()
	}
	for _, c := range nfcs {
		c.StopTail()
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
	pts := make([]influxdb.Point, 0, 100)
	defer func() {
		log.Println("[INFO] exiting influxdb routine!")
		close(done)
	}()

	timeout := time.After(INFLUXDB_BUFFER_DUR * time.Millisecond)
	for {
		select {
		case point, more := <-influxchan:
			if !more {
				if len(pts) != 0 {
					sendBatch(pts)
				}

				return
			}

			pts = append(pts, *point)
		case <-timeout:
			sendBatch(pts)
			pts = pts[:0]
			timeout = time.After(INFLUXDB_BUFFER_DUR * time.Millisecond)
		}
	}
}

func dockerListener(docker *dockerclient.Client, dokchan chan *dockerclient.APIEvents, done chan bool) {
	log.Println("[INFO] listening to docker container events")
	defer func() {
		log.Println("[INFO] exiting docker listener!")
		close(done)
	}()

	influxchan := make(chan *influxdb.Point, INFLUXDB_BUFFER_SIZE)
	donechan := make(chan bool)
	go bgWrite(influxchan, donechan)
	defer func() {
		close(influxchan)
		<-donechan
	}()

	defer func() { cleanupConts() }()
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

				volume := cont.Volumes[SIPP_PATH_VOL]
				if volume == "" {
					for _, v := range cont.Mounts {
						if v.Destination == SIPP_PATH_VOL {
							volume = v.Source
							break
						}
					}
				}
				if volume == "" {
					log.Println("[WARN] unable to find volume for container ", event.ID)
					continue
				}

				c := &SippCont{
					id:       cont.Name[1:],
					vid:      volume,
					stopchan: make(chan bool),
				}
				sippvols[event.ID] = c
				go c.Tail(influxchan, cont.State.Pid)
				log.Printf("[INFO] add volume %s to map for container %s\n", volume, event.ID)
			} else if event.From == IMAGE_SNORT || event.From == IMAGE_SURICATA {
				cont, err := docker.InspectContainer(event.ID)
				if err != nil {
					log.Println("[WARN] unable to inspect container ", event.ID)
					continue
				}

				c := &NFCont{
					id:       cont.Name[1:],
					pid:      cont.State.Pid,
					stopchan: make(chan bool),
					waitchan: make(chan bool),
				}

				nfcs[event.ID] = c
				go c.Tail(influxchan)
				log.Printf("[INFO] add into tail list container %s\n", event.ID)
			}
		case "die", "kill", "stop":
			if c, ok := sippvols[event.ID]; ok {
				c.StopTail()
				delete(sippvols, event.ID)
				log.Printf("[INFO] delete container %s\n", event.ID)
				continue
			}

			if c, ok := nfcs[event.ID]; ok {
				c.StopTail()
				delete(nfcs, event.ID)
				log.Printf("[INFO] delete container %s\n", event.ID)
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
		log.Println("[ERROR] unable to get system id:", err)
		panic(err)
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
		log.Println("[ERROR] no client is parsable, exiting!")
		panic(true)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	log.Println("[INFO] adding signal handler for SIGTERM")

	docker, err := dockerclient.NewClientFromEnv()
	if err != nil {
		log.Println("[ERROR] unable to create docker client, exiting!")
		panic(err)
	}

	dokchan := make(chan *dockerclient.APIEvents, 100)
	waitchan := make(chan bool)
	defer func() {
		close(dokchan)
		<-waitchan
	}()

	err = docker.AddEventListener(dokchan)
	if err != nil {
		log.Println("[ERROR] unable to add docker event listener, exiting!")
		panic(err)
	}
	defer docker.RemoveEventListener(dokchan)
	go dockerListener(docker, dokchan, waitchan)

	_ = <-sigs
	log.Println("[INFO] goodbye!")
}
