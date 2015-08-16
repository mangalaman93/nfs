package common

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

func TotalMem() int64 {
	out, err := exec.Command("free", "-b").Output()
	if err != nil {
		log.Fatalln("[ERROR] Unable to execute free, ", err.Error())
	}

	mem := strings.Fields(fmt.Sprintf("%s", out))[7]
	val, err := strconv.ParseInt(mem, 10, 64)
	if err != nil {
		log.Fatalln("[ERROR] Unable to find total memory, ", err.Error())
	}

	return val
}
