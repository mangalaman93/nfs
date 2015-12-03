package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	dockerclient "github.com/fsouza/go-dockerclient"
)

type NFCont struct {
	id       string
	pid      int
	stopchan chan bool
	wg       sync.WaitGroup
	dbclient *DBClient
}

func NewNFCont(id string, pid int, dbclient *DBClient) *NFCont {
	return &NFCont{
		id:       id,
		pid:      pid,
		stopchan: make(chan bool),
		dbclient: dbclient,
	}
}

func (n *NFCont) String() string {
	return fmt.Sprintf("{id:%s, pid:%s}", n.id, n.pid)
}

func (n *NFCont) Tail(docker *dockerclient.Client) {
	n.wg.Add(1)
	go n.tail(docker)
}

func (n *NFCont) StopTail() {
	close(n.stopchan)
	n.wg.Wait()
	log.Println("[INFO] cleaned up for container", n.id)
}

func (n *NFCont) tail(docker *dockerclient.Client) {
	defer n.wg.Done()

	// wait for queue to be created
	netfilter_file := path.Join(HOST_PROC_PATH, strconv.Itoa(n.pid), "/net/netfilter/nfnetlink_queue")
	dev_file := path.Join(HOST_PROC_PATH, strconv.Itoa(n.pid), "/net/dev")
	for {
		timeout := time.After(FILES_CHECK_INTERVAL * time.Millisecond)
		select {
		case <-n.stopchan:
			log.Println("[INFO] no clean up required for containern", n.id)
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
		case <-n.stopchan:
			log.Println("[INFO] exiting Tail for container", n.id)
			return
		default:
			time.Sleep(time.Duration(READ_PERIOD) * time.Millisecond)
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
		if ival, err := strconv.ParseInt(row[2], 10, 64); err != nil {
			log.Println("[WARN] unable to parse", row[2], "err:", err)
		} else {
			n.dbclient.Write("snort_queue_length", n.id, map[string]interface{}{"value": ival}, curtime)
		}
		if ival, err := strconv.ParseInt(row[5], 10, 64); err != nil {
			log.Println("[WARN] unable to parse", row[5], "err:", err)
		} else {
			n.dbclient.Write("snort_queue_drops", n.id, map[string]interface{}{"value": ival}, curtime)
		}
		if ival, err := strconv.ParseInt(row[6], 10, 64); err != nil {
			log.Println("[WARN] unable to parse", row[6], "err:", err)
		} else {
			n.dbclient.Write("snort_user_drops", n.id, map[string]interface{}{"value": ival}, curtime)
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
			log.Println("[WARN] incorrect number of lines in dev file for container", n.id)
			continue
		}
		index := 3
		if strings.Contains(rows[2], "lo") {
			if strings.Contains(rows[3], "lo") {
				log.Println("[WARN] unable to find correct network interface for container", n.id)
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
		if ival, err := strconv.ParseInt(row[2], 10, 64); err != nil {
			log.Println("[WARN] unable to parse", row[2], "err:", err)
		} else {
			n.dbclient.Write("rx_packets", n.id, map[string]interface{}{"value": ival}, curtime)
		}
		if ival, err := strconv.ParseInt(row[10], 10, 64); err != nil {
			log.Println("[WARN] unable to parse", row[10], "err:", err)
		} else {
			n.dbclient.Write("tx_packets", n.id, map[string]interface{}{"value": ival}, curtime)
		}

		// available shares
		cont, err := docker.InspectContainer(n.id)
		if err != nil {
			log.Println("[WARN] unable to inspect container", n.id)
			continue
		}
		n.dbclient.Write("available_shares", n.id, map[string]interface{}{"value": cont.HostConfig.CPUShares}, time.Now())
	}

	log.Println("[INFO] exiting Tail for container", n.id)
}
