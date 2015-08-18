package docker

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
)

import (
	"github.com/mangalaman93/nfs/linux"
)

func ListContainers() ([]string, error) {
	out, err := linux.Exec("docker ps -q --no-trunc")
	if err != nil {
		return nil, err
	}

	return strings.Fields(out), nil
}

func GetCPUUsage(cont string) (int64, error) {
	cpuacct_file := fmt.Sprintf("/sys/fs/cgroup/cpuacct/system.slice/docker-%s.scope/cpuacct.stat", cont)
	data, err := ioutil.ReadFile(cpuacct_file)
	if err != nil {
		return 0, err
	}

	sdata := strings.Fields(string(data))
	user_cpu, err := strconv.ParseInt(sdata[1], 10, 64)
	if err != nil {
		return 0, err
	}
	sys_cpu, err := strconv.ParseInt(sdata[3], 10, 64)
	if err != nil {
		return 0, err
	}

	return (user_cpu + sys_cpu), nil
}

func GetMemUsage(cont string) (int64, error) {
	cmd := fmt.Sprintf("head -2 /sys/fs/cgroup/memory/system.slice/docker-%s.scope/memory.stat", cont)
	out, err := linux.Exec(cmd)
	if err != nil {
		return 0, err
	}

	mem, err := strconv.ParseInt(strings.Fields(out)[3], 10, 64)
	if err != nil {
		return 0, err
	}

	return mem, nil
}

func GetNetOutUsage(cont string) (int64, error) {
	cmd := fmt.Sprintf("docker exec %s cat /sys/devices/virtual/net/eth0/statistics/tx_bytes", cont)
	out, err := linux.Exec(cmd)
	if err != nil {
		return 0, err
	}

	net, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return 0, err
	}

	return net, nil
}

func GetNetInUsage(cont string) (int64, error) {
	cmd := fmt.Sprintf("docker exec %s cat /sys/devices/virtual/net/eth0/statistics/rx_bytes", cont)
	out, err := linux.Exec(cmd)
	if err != nil {
		return 0, err
	}

	net, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return 0, err
	}

	return net, nil
}
