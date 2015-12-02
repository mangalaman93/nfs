package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mangalaman93/tail"
)

const (
	SIPP_UDP_PORT    = 5060
	STAT_FIELD_COUNT = 87
	MAX_LINE_LENGTH  = 3000
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

type SippCont struct {
	id       string
	vid      string
	pid      int
	rtt      *tail.Tail
	stat     *tail.Tail
	stopchan chan bool
	wg       sync.WaitGroup
	dbclient *DBClient
}

func NewSippCont(id, vid string, pid int, dbclient *DBClient) *SippCont {
	return &SippCont{
		id:       id,
		vid:      vid,
		pid:      pid,
		stopchan: make(chan bool),
		dbclient: dbclient,
	}
}

func (s *SippCont) String() string {
	return fmt.Sprintf("{id:%s, vid:%s, rtt:%s, stat:%s}", s.id, s.vid, s.rtt, s.stat)
}

func (s *SippCont) Tail() {
	s.wg.Add(1)
	go s.tail()
}

func (s *SippCont) StopTail() {
	close(s.stopchan)
	s.wg.Wait()
	log.Println("[INFO] cleaned up for container", s.id)
}

func (s *SippCont) tail() {
	defer s.wg.Done()

	var files []string
	var err error
	for {
		timeout := time.After(FILES_CHECK_INTERVAL * time.Millisecond)
		select {
		case <-s.stopchan:
			log.Println("[INFO] no clean up required for container", s.id)
			return
		case <-timeout:
		}

		files, err = filepath.Glob(filepath.Join(s.vid, "*.csv"))
		if err != nil {
			log.Printf("[WARN] unable to find files for container %s\n", s.id)
			continue
		}
		if len(files) == 2 {
			break
		} else if len(files) > 2 {
			log.Println("[WARN] more than 2 .csv files present for container", s.id)
			return
		} else {
			log.Println("[WARN] less than 2 .csv files present for container", s.id)
		}
	}

	s.stat, err = tail.TailFile(files[0], MAX_LINE_LENGTH)
	if err != nil {
		log.Println("[WARN] unable to read stat file for container", s.id)
	} else {
		s.wg.Add(1)
		go s.dispatchStats()
	}
	s.rtt, err = tail.TailFile(files[1], MAX_LINE_LENGTH)
	if err != nil {
		log.Println("[WARN] unable to read rtt file for container", s.id)
	} else {
		s.wg.Add(1)
		go s.dispatchRtts()
	}
	// go routine to collect UDP queue size
	s.wg.Add(1)
	go s.tailUDP()

	// wait for stop signal and then stop tailing files instead of doing the same in StopTail()
	// function so that in case StopTail is called even before control of the program reached here
	// i.e. if s.rtt and s.stat are not initialised, calling Stop() will not result in segmentation fault
	<-s.stopchan
	s.rtt.Stop()
	s.stat.Stop()
}

func (s *SippCont) dispatchRtts() {
	defer s.wg.Done()
	for _ = range s.rtt.Lines {
		break
	}

	ticker := time.NewTicker(1 * time.Second).C
	sum := 0.0
	count := 0
	for line := range s.rtt.Lines {
		select {
		case <-ticker:
			if count == 0 {
				break
			}

			s.dbclient.Write("response_time", s.id, map[string]interface{}{
				"value": sum / float64(count),
			}, time.Now())
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

	log.Printf("[INFO] exiting dispatchRtts for container %s with error %s\n", s.id, s.rtt.Err)
}

func (s *SippCont) dispatchStats() {
	defer s.wg.Done()
	for _ = range s.stat.Lines {
		break
	}

	for line := range s.stat.Lines {
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
				continue
			}
			s.dbclient.Write(measurement, s.id, map[string]interface{}{
				"value": value,
			}, curtime)
		}
	}

	log.Printf("[INFO] exiting dispatchStats for container %s with error: %s\n", s.id, s.stat.Err)
}

func (s *SippCont) tailUDP() {
	defer s.wg.Done()

	hex_port := strings.ToUpper(fmt.Sprintf("%x", SIPP_UDP_PORT))
	udp_file := path.Join(HOST_PROC_PATH, strconv.Itoa(s.pid), "/net/udp")
	if _, err := os.Stat(udp_file); err != nil {
		log.Printf("[ERROR] %s file not found!\n", udp_file)
		panic(err)
	}

	for {
		select {
		case <-s.stopchan:
			log.Println("[INFO] exiting tailUDP for container", s.id)
			return
		default:
			time.Sleep(time.Duration(READ_PERIOD) * time.Millisecond)
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
			log.Println("[WARN] unable to find udp port for container", s.id)
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

		if ival, err := strconv.ParseInt(rx_tx[0], 16, 64); err != nil {
			log.Println("[WARN] unable to parse", rx_tx[0], "err:", err)
		} else {
			s.dbclient.Write("txqueue_udp", s.id, map[string]interface{}{"value": ival}, curtime)
		}
		if ival, err := strconv.ParseInt(rx_tx[1], 16, 64); err != nil {
			log.Println("[WARN] unable to parse", rx_tx[1], "err:", err)
		} else {
			s.dbclient.Write("rxqueue_udp", s.id, map[string]interface{}{"value": ival}, curtime)
		}
		if ival, err := strconv.ParseInt(row[12], 10, 64); err != nil {
			log.Println("[WARN] unable to parse", row[12], "err:", err)
		} else {
			s.dbclient.Write("drops_udp", s.id, map[string]interface{}{"value": ival}, curtime)
		}
	}
}
