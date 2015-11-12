package nfsmain

import (
	"log"

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
	vl.Start()

	apps = make(map[string]AppLine)
	apps[vl.GetDB()] = vl

	go ListenLine(port, apps)
	return nil
}

func Stop() {
	for _, app := range apps {
		app.Stop()
	}

	// for now, we don't know how to stop http listener
	// and we do not want to get into complications of
	// creating listener ourselves (later)
	log.Println("[_INFO] exiting control loop")
}
