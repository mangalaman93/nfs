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

// return total docker container cpu usage
func GetCPUUsage(cont_id string) (int64, error) {
	cpuacct_file := fmt.Sprintf("/sys/fs/cgroup/cpuacct/system.slice/docker-%s.scope/cpuacct.stat", cont_id)
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
