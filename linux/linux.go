package linux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func Exec(cmd string) (string, error) {
	parts := strings.Fields(cmd)
	command := exec.Command(parts[0], parts[1:]...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), err
	}

	return string(output), nil
}

func TotalMem() (int64, error) {
	out, err := Exec("free -b")
	if err != nil {
		return 0, err
	}

	mem := strings.Fields(fmt.Sprintf("%s", out))[7]
	val, err := strconv.ParseInt(mem, 10, 64)
	if err != nil {
		return 0, err
	}

	return val, nil
}
