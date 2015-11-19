package nfsmain

import (
	"log"
	"net/http"

	"github.com/Unknwon/goconfig"
	"github.com/mangalaman93/nfs/voip"
)

var (
	apps map[string]AppLine
)

func Start(config *goconfig.ConfigFile) error {
	port, err := config.GetValue("LINE_PROTOCOL", "port")
	if err != nil {
		return err
	}

	vl, err := voip.NewVoipLine(config)
	if err != nil {
		return err
	}

	h, err := NewHandler(config, apps)
	if err != nil {
		return err
	}

	apps = make(map[string]AppLine)
	apps[vl.GetDB()] = vl
	log.Println("[_INFO] registered db:", vl.GetDB(), "with VoipLine instance")

	go vl.Start()
	go http.ListenAndServe(":"+port, h)
	log.Println("[_INFO] listening for data over line protocol on port", port)
	return nil
}

func Stop() {
	for _, app := range apps {
		app.Stop()
	}

	// TODO: for now, we don't know how to stop http
	// listener and we do not want to get into
	// complications of creating listener ourselves
	log.Println("[_INFO] exiting control loop")
}
