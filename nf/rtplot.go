package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// return total docker container cpu usage
func get_cum_cpu_usage(cont_id string) int64 {
	cpuacct_file := fmt.Sprintf("/sys/fs/cgroup/cpuacct/system.slice/docker-%s.scope/cpuacct.stat", cont_id)
	data, err := ioutil.ReadFile(cpuacct_file)
	if err != nil {
		fmt.Println("[ERROR] error while reading cpuacct cgroup file!")
		panic(err)
	}

	sdata := strings.Fields(string(data))
	user_cpu, err := strconv.ParseInt(sdata[1], 10, 64)
	if err != nil {
		fmt.Println("[ERROR], error while converting cpu usage to integer!")
		panic(err)
	}
	sys_cpu, err := strconv.ParseInt(sdata[3], 10, 64)
	if err != nil {
		fmt.Println("[ERROR], error while converting cpu usage to integer!")
		panic(err)
	}

	return (user_cpu + sys_cpu)
}

func main() {
	// read arguments
	if len(os.Args) < 3 {
		fmt.Println("[ERROR] incomplete command!")
		fmt.Printf("Usage: %s <cont-name> <freq>\n", os.Args[0])
		os.Exit(1)
	}
	cont_name := os.Args[1]
	freq, err := strconv.ParseInt(os.Args[2], 10, 32)
	if err != nil {
		fmt.Println("[ERROR] unknown frequency argument!")
		os.Exit(1)
	}

	// get container id
	cmd := fmt.Sprintf("docker inspect --format='{{.Id}}' %s", cont_name)
	parts := strings.Fields(cmd)
	command := exec.Command(parts[0], parts[1:]...)
	out, err := command.CombinedOutput()
	if err != nil {
		fmt.Printf("[ERROR] unable to run cmd, output: (%s)\n", out)
		panic(err)
	}
	cont_id := strings.TrimSpace(string(out))

	// check if feedgnuplot exists
	command = exec.Command("which", "feedgnuplot")
	out, err = command.CombinedOutput()
	if err != nil || string(out) == "" {
		panic("[ERROR] feedgnuplot doesn't exists!")
	}

	// CTRL+C signal handler
	sigs := make(chan os.Signal, 1)
	done := true
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_ = <-sigs
		done = false
	}()

	// init feedgnuplot
	command = exec.Command("feedgnuplot", "--lines --stream --title 'cpu-usage' --ylabel 'cpu (100% = 1core)' --xlabel 'time ticks'")
	reader, writer := io.Pipe()
	command.Stdin = reader
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Start()
	if err != nil {
		fmt.Println("[ERROR] error while starting feedgnuplot!")
		panic(err)
	}

	// init readings
	cur_time := time.Now().UnixNano()
	cur_cpu := get_cum_cpu_usage(cont_id)

	// infinite loop for plotting data
	for done {
		new_cpu := get_cum_cpu_usage(cont_id)
		new_time := time.Now().UnixNano()
		avg_cpu := float64(new_cpu-cur_cpu) / float64(new_time-cur_time) * 1000000000.0
		cur_time = new_time
		cur_cpu = new_cpu

		_, err = io.WriteString(writer, fmt.Sprintf("%.6f\n", avg_cpu))
		if err != nil {
			panic("[ERROR] unable to write to feedgnuplot-stdin!")
		}

		time.Sleep(time.Duration(freq) * time.Millisecond)
	}

	_, err = io.WriteString(writer, "exit\n")
	if err != nil {
		panic("[ERROR] unable to write to feedgnuplot-stdin!")
	}

	command.Wait()
	return
}
