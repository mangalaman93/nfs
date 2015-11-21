/* plots real time graph of cpu usage of
   a container using feedgnuplot
*/

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

import (
	"github.com/mangalaman93/collectdocker/docker"
	"github.com/mangalaman93/collectdocker/linux"
)

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
	out, err := linux.Exec(fmt.Sprintf("docker inspect --format='{{.Id}}' %s", cont_name))
	if err != nil {
		fmt.Printf("[ERROR] unable to run cmd, output: (%s)\n", out)
		panic(err)
	}
	cont_id := strings.TrimSpace(string(out))

	// check if feedgnuplot exists
	command := exec.Command("which", "feedgnuplot")
	empty, err := command.CombinedOutput()
	if err != nil || string(empty) == "" {
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
	cur_cpu, err := docker.GetCPUUsage(cont_id)
	if err != nil {
		fmt.Println("[WARN] unable to get container CPU usage!")
		cur_cpu = 0
	}

	// infinite loop for plotting data
	for done {
		new_cpu, err := docker.GetCPUUsage(cont_id)
		if err != nil {
			fmt.Println("[WARN] unable to get container CPU usage!")
			new_cpu = 0
		}

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
