package main

import (
	"os"
)

const (
	FILES_CHECK_INTERVAL = 4000
	READ_PERIOD          = 1000
)

var (
	HOST_PROC_PATH = "/proc"
)

func InitConfig() {
	proc_path := os.Getenv("PROC_PATH")
	if proc_path != "" {
		HOST_PROC_PATH = proc_path
	}
}
