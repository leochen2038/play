package server

import (
	"fmt"
	"github.com/leochen2038/play"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const (
	envGraceful = "GRACEFUL"
)

var (
	ppidkilled   bool
	wg           sync.WaitGroup
	playListener net.Listener
	httpListener net.Listener
)

type filer interface {
	File() (*os.File, error)
}

func init() {
	go signalHandler()
}

func reload() (int, error) {
	var err error
	var tag = "-"
	var sockes = make([]*os.File, 0, 1)
	var originalWD, _ = os.Getwd()

	defer func() {
		for _, v := range sockes {
			v.Close()
		}
	}()

	if playListener == nil {
		tag += "0"
	} else {
		tag += "1"
		socket, _ := playListener.(filer).File()
		sockes = append(sockes, socket)
	}

	if httpListener == nil {
		tag += "0"
	} else {
		tag += "1"
		socket, _ := httpListener.(filer).File()
		sockes = append(sockes, socket)
	}

	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return 0, err
	}

	var env []string
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, envGraceful) {
			env = append(env, v)
		}
	}

	env = append(env, fmt.Sprintf("%s=%s%s", envGraceful, strings.ToLower(envGraceful), tag))
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	files = append(files, sockes...)

	process, err := os.StartProcess(argv0, os.Args, &os.ProcAttr{
		Dir:   originalWD,
		Env:   env,
		Files: files,
	})
	if err != nil {
		return 0, err
	}
	return process.Pid, nil
}

func signalHandler() {
	ch := make(chan os.Signal, 10)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2)

	for {
		switch <-ch {
		case syscall.SIGINT, syscall.SIGTERM:
			signal.Stop(ch)
			play.CronStop()
			wg.Wait()
			os.Exit(0)
		case syscall.SIGUSR2:
			if _, err := reload(); err != nil {
				fmt.Println("reload error:", err.Error())
			}
		}
	}
}

func shouldKillParent() (err error) {
	if !ppidkilled && os.Getppid() != 1 {
		ppidkilled = true
		if err := syscall.Kill(os.Getppid(), syscall.SIGTERM); err == nil {
			log.Printf("[play server] graceful handoff success with new pid %d", os.Getpid())
		}
	}
	return
}

func getGracefulSocket(x int) uintptr {
	var id uintptr = 2
	if os.Getenv(envGraceful) != "" && len(os.Getenv(envGraceful)) >= 9+x {
		for _, v := range os.Getenv(envGraceful)[9 : 9+x] {
			if v == 49 {
				id++
			}
		}
	}
	return id
}
