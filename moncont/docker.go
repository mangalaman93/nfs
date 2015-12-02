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
		switch {
		case event.Status == "start" && event.From == IMAGE_SIPP:
			if _, ok := h.sippvols[event.ID]; ok {
				log.Println("[WARN] duplicate event for container", event.ID)
				continue
			}
			cont, err := h.docker.InspectContainer(event.ID)
			if err != nil {
				log.Println("[WARN] unable to inspect container", event.ID)
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
				log.Println("[WARN] unable to find volume for container", event.ID)
				continue
			}
			scont := NewSippCont(cont.Name[1:], volume, cont.State.Pid, h.dbclient)
			h.sippvols[event.ID] = scont
			scont.Tail()
			log.Println("[INFO] monitoring container", event.ID)
		case event.Status == "start" && (event.From == IMAGE_SNORT || event.From == IMAGE_SURICATA):
			if _, ok := h.nfconts[event.ID]; ok {
				log.Println("[WARN] duplicate event for container", event.ID)
				continue
			}
			cont, err := h.docker.InspectContainer(event.ID)
			if err != nil {
				log.Println("[WARN] unable to inspect container", event.ID)
				continue
			}
			nfcont := NewNFCont(cont.Name[1:], cont.State.Pid, h.dbclient)
			h.nfconts[event.ID] = nfcont
			nfcont.Tail()
			log.Println("[INFO] monitoring container", event.ID)
		case event.Status == "die" || event.Status == "kill" || event.Status == "stop":
			if scont, ok := h.sippvols[event.ID]; ok {
				scont.StopTail()
				delete(h.sippvols, event.ID)
				log.Printf("[INFO] delete container %s\n", event.ID)
				continue
			}
			if nfcont, ok := h.nfconts[event.ID]; ok {
				nfcont.StopTail()
				delete(h.nfconts, event.ID)
				log.Printf("[INFO] delete container %s\n", event.ID)
			}
		}
	}

	log.Println("Exiting docker events listener loop!")
}
