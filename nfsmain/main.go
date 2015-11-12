package nfsmain

import (
	"log"

	"github.com/Unknwon/goconfig"
)

var (
	sdata State
)

func Start(config *goconfig.ConfigFile) error {
	db, err := config.GetValue("INFLUXDB", "db")
	if err != nil {
		return err
	}

	port, err := config.GetValue("INFLUXDB", "port")
	if err != nil {
		return err
	}

	go ListenLine(db, port)
	return nil
}

func Stop() {
	// for now, we don't know how to stop http listener
	// and we do not want to get into complications of
	// creating listener ourselves (later)
	log.Println("[INFO] exiting control loop")
}
