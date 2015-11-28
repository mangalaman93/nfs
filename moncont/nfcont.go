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

func (n *NFCont) Tail() {
	n.wg.Add(1)
	go n.tail()
}

func (n *NFCont) StopTail() {
	close(n.stopchan)
	n.wg.Wait()
	log.Println("[INFO] cleaned up for container", n.id)
}

func (n *NFCont) tail() {
	defer n.wg.Done()

	// wait for queue to be created
	netfilter_file := path.Join("/proc/", strconv.Itoa(n.pid), "/net/netfilter/nfnetlink_queue")
	for {
		timeout := time.After(FILES_CHECK_INTERVAL * time.Millisecond)
		select {
		case <-n.stopchan:
			log.Println("[INFO] no clean up required for containern", n.id)
			return
		case <-timeout:
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
	}

	log.Println("[INFO] exiting Tail for container", n.id)
}
