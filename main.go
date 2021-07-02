package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

func main() {
	start := time.Now()
	var (
		MAX_RATE int
		INFLIGHT int
		wg       sync.WaitGroup
	)
	flag.IntVar(&MAX_RATE, "rate", runtime.NumCPU(), "max rate")
	flag.IntVar(&INFLIGHT, "inflight", runtime.NumCPU(), "max inflight")
	flag.Parse()
	command := flag.Arg(0)
	stdInVar := flag.Arg(1)

	rate_chan := make(chan struct{}, MAX_RATE)
	timeout := time.Tick(time.Second * 1)

	go func() {
		for {
			<-timeout
			for _ = range rate_chan {
				<-rate_chan
			}
		}
	}()

	if stdInVar == "{}" {
		rate_chan <- struct{}{}
		info, err := os.Stdin.Stat()
		if err != nil {
			panic(err)
		}

		if info.Mode()&os.ModeCharDevice != 0 || info.Size() <= 0 {
			return
		}
		buf := make([]byte, 128)
		reader := bufio.NewReader(os.Stdin)

		inflight_chan := make(chan struct{}, INFLIGHT)
		args_chan := make(chan string, 128)

		for {
			_, err := reader.Read(buf)
			if err != nil && err == io.EOF {
				break
			}
			if string(buf) == "\n" {
				continue
			}

			args_chan <- string(buf)
		}

		wg.Add(len(args_chan))
		for i := 0; i < len(args_chan); i++ {
			inflight_chan <- struct{}{}

			go func(command string, wg *sync.WaitGroup, inflight_chan chan struct{}, args_chan chan string) {
				defer func() {
					<-inflight_chan
					wg.Done()
				}()

				arg := <-args_chan
				arg = strings.TrimSpace(arg)
				command = strings.TrimSpace(command)
				runCommand(arg, command)
			}(command, &wg, inflight_chan, args_chan)
		}
	}
	wg.Wait()
	fmt.Println("EXECUTION TIME:", time.Since(start))
}

//В горутинах вызываем эту функцию, и исполняем команду
func runCommand(arg string, command string) {
	cmd := exec.Command(command, arg)
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		return
	}
}
