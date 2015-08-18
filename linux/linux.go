package linux

import (
	"bufio"
	"errors"
	"fmt"
	"os"
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

func GetCPUUsage() (int64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line[4:])

		var total_cpu int64
		for _, num := range fields {
			cpu, err := strconv.ParseInt(num, 10, 64)
			if err != nil {
				return 0, err
			}
			total_cpu += cpu
		}

		idle_cpu, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return 0, err
		}

		return (total_cpu - idle_cpu), nil
	}

	return 0, errors.New("unreachable code")
}

func GetMemUsage() (int64, error) {
	out, err := Exec("free -b")
	if err != nil {
		return 0, err
	}

	i := strings.Index(out, "buffers/cache")
	mem, err := strconv.ParseInt(strings.Fields(out[i:])[1], 10, 64)
	if err != nil {
		return 0, err
	}

	return mem, nil
}
