package main

import (
	"log"
	"sync"

	dockerclient "github.com/fsouza/go-dockerclient"
)

const (
	IMAGE_SIPP     = "mangalaman93/sipp"
	IMAGE_SNORT    = "mangalaman93/snort"
	IMAGE_SURICATA = "mangalaman93/suricata"
	SIPP_PATH_VOL  = "/data"
)

type DockerHandler struct {
	sippvols map[string]*SippCont
	nfconts  map[string]*NFCont
	dbclient *DBClient
	docker   *dockerclient.Client
	echan    chan *dockerclient.APIEvents
	stopchan chan bool
	wg       sync.WaitGroup
}

func NewDockerHandler(dbclient *DBClient) (*DockerHandler, error) {
	docker, err := dockerclient.NewClientFromEnv()
	if err != nil {
		return nil, err
	}

	return &DockerHandler{
		sippvols: make(map[string]*SippCont),
		nfconts:  make(map[string]*NFCont),
		dbclient: dbclient,
		docker:   docker,
		echan:    make(chan *dockerclient.APIEvents, 100),
		stopchan: make(chan bool),
	}, nil
}

func (h *DockerHandler) Start() error {
	err := h.docker.AddEventListener(h.echan)
	if err != nil {
		return err
	}

	h.wg.Add(1)
	go h.listen()
	return nil
}

func (h *DockerHandler) Stop() {
	h.docker.RemoveEventListener(h.echan)
	close(h.stopchan)
	close(h.echan)
	h.wg.Wait()

	for _, s := range h.sippvols {
		s.StopTail()
	}
	for _, c := range h.nfconts {
		c.StopTail()
	}
}

func (h *DockerHandler) listen() {
	defer h.wg.Done()
	for event := range h.echan {
		log.Println("[INFO] event occurred:", event)
		h.handleEvent(event)
	}
}

func (h *DockerHandler) handleEvent(event *dockerclient.APIEvents) {
	switch {
	case event.Status == "start":
		h.handleStartEvent(event)
	case event.Status == "die" || event.Status == "kill" || event.Status == "stop":
		h.handleStopEvent(event)
	}
}

func (h *DockerHandler) handleStartEvent(event *dockerclient.APIEvents) {
	switch event.From {
	case IMAGE_SIPP:
		h.wg.Add(1)
		go h.monitorResource(event)
		if _, ok := h.sippvols[event.ID]; ok {
			log.Println("[WARN] duplicate event for container", event.ID)
			return
		}
		cont, err := h.docker.InspectContainer(event.ID)
		if err != nil {
			log.Println("[WARN] unable to inspect container", event.ID)
			return
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
			log.Println("[WARN] unable to find volume for container", event.ID)
			return
		}
		scont := NewSippCont(cont.Name[1:], volume, cont.State.Pid, h.dbclient)
		h.sippvols[event.ID] = scont
		scont.Tail()
		log.Println("[INFO] monitoring container", event.ID)
	case IMAGE_SNORT:
	case IMAGE_SURICATA:
		h.wg.Add(1)
		go h.monitorResource(event)
		if _, ok := h.nfconts[event.ID]; ok {
			log.Println("[WARN] duplicate event for container", event.ID)
			return
		}
		cont, err := h.docker.InspectContainer(event.ID)
		if err != nil {
			log.Println("[WARN] unable to inspect container", event.ID)
			return
		}
		nfcont := NewNFCont(cont.Name[1:], cont.State.Pid, h.dbclient)
		h.nfconts[event.ID] = nfcont
		nfcont.Tail()
		log.Println("[INFO] monitoring container", event.ID)
	}
}

func (h *DockerHandler) handleStopEvent(event *dockerclient.APIEvents) {
	if scont, ok := h.sippvols[event.ID]; ok {
		scont.StopTail()
		delete(h.sippvols, event.ID)
		log.Println("[INFO] delete container", event.ID)
		return
	}
	if nfcont, ok := h.nfconts[event.ID]; ok {
		nfcont.StopTail()
		delete(h.nfconts, event.ID)
		log.Println("[INFO] delete container", event.ID)
	}
}

func (h *DockerHandler) monitorResource(event *dockerclient.APIEvents) {
	defer h.wg.Done()

	// go routine for listening for stats
	statchan := make(chan *dockerclient.Stats)
	h.wg.Add(1)
	go h.listenForStats(event, statchan)

	// go routine for signaling Stats function
	// TODO: this is incorrect, as this routine won't stop even when
	// container has stopped. The stopchan should be assigned per container
	donechan := make(chan bool)
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		<-h.stopchan
		close(donechan)
	}()

	// listening for stats
	retErr := h.docker.Stats(dockerclient.StatsOptions{
		ID:     event.ID,
		Stats:  statchan,
		Stream: true,
		Done:   donechan,
	})
	log.Println("[INFO] exiting monitorResource for container", event.ID, "with err", retErr)
}

func (h *DockerHandler) listenForStats(event *dockerclient.APIEvents, statchan chan *dockerclient.Stats) {
	defer h.wg.Done()
	for stat := range statchan {
		h.dbclient.Write("rx_bytes", event.ID, map[string]interface{}{"value": stat.Network.RxBytes}, stat.Read)
		h.dbclient.Write("tx_bytes", event.ID, map[string]interface{}{"value": stat.Network.TxBytes}, stat.Read)
		h.dbclient.Write("rx_packets", event.ID, map[string]interface{}{"value": stat.Network.RxPackets}, stat.Read)
		h.dbclient.Write("tx_packets", event.ID, map[string]interface{}{"value": stat.Network.TxPackets}, stat.Read)
		h.dbclient.Write("cpu_usage_total", event.ID, map[string]interface{}{"value": stat.CPUStats.CPUUsage.TotalUsage}, stat.Read)
	}
}
