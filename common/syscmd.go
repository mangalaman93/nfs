package common

import (
	"os/exec"
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
