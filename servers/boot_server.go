package servers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/leochen2038/play"
	"golang.org/x/sync/errgroup"
)

type runningInstance struct {
	server      play.IServer
	listener    net.Listener
	udpListener net.PacketConn
}

var (
	instances  sync.Map
	runs       []string
	ppidkilled bool
)

const (
	envGraceful = "GRACEFUL"
)

func init() {
	go signalHandler()
}

type filer interface {
	File() (*os.File, error)
}

func Boot(is ...play.IServer) error {
	var instanceWaitGroup sync.WaitGroup
	var egr errgroup.Group
	for _, i := range is {
		if i != nil {
			var i = i
			var err error
			var listener net.Listener
			var udplistener net.PacketConn
			var gracefulSocket = getGracefulSocket(i.Info().Name)
			egr.Go(func() error {
				switch i.Network() {
				case "tcp":
					if gracefulSocket > 0 {
						if listener, err = net.FileListener(os.NewFile(gracefulSocket, "")); err != nil {
							return err
						}
					} else {
						if listener, err = net.Listen(i.Network(), i.Info().Address); err != nil {
							return err
						}
					}
				case "udp":
					if gracefulSocket > 0 {
						if udplistener, err = net.FilePacketConn(os.NewFile(gracefulSocket, "")); err != nil {
							return err
						}
					} else {
						if udplistener, err = net.ListenPacket(i.Network(), i.Info().Address); err != nil {
							return err
						}
					}
				default:
					return errors.New("unsupported network")
				}

				if _, ok := instances.Load(i.Info().Name); ok {
					if listener != nil {
						_ = listener.Close()
					}
					if udplistener != nil {
						_ = udplistener.Close()
					}
					return errors.New("server name " + i.Info().Name + " is running")
				}

				instanceWaitGroup.Add(1)
				instances.Store(i.Info().Name, runningInstance{listener: listener, udpListener: udplistener, server: i})
				runs = append(runs, i.Info().Name)
				go func() {
					defer instanceWaitGroup.Done()
					_ = i.Run(listener, udplistener)
				}()
				i.Hook().OnBoot(i)
				return nil
			})
		}
	}
	if err := egr.Wait(); err != nil {
		for _, i := range is {
			if i != nil {
				Shutdown(i.Info().Name)
			}
		}
		return err
	}
	if os.Getenv(envGraceful) != "" {
		if err := shouldKillParent(); err != nil {
			os.Exit(1)
		}
	}

	instanceWaitGroup.Wait()
	return nil
}

func ShutdownAll() {
	for _, v := range runs {
		Shutdown(v)
	}
}

func Shutdown(name string) {
	if v, ok := instances.Load(name); ok {
		instances.Delete(name)
		i := v.(runningInstance)

		defer func() {
			i.server.Close()
			if i.listener != nil {
				_ = i.listener.Close()
			}
			if i.udpListener != nil {
				_ = i.udpListener.Close()
			}
			if panicInfo := recover(); panicInfo != nil {
				fmt.Printf("panic: %v\n%v", panicInfo, string(debug.Stack()))
			}
		}()
		i.server.Hook().OnShutdown(i.server)
	}
}

// 返回callAction里的onFinish错误
func doRequest(gctx context.Context, s *play.Session, request *play.Request) (err error) {
	s.Server.Ctrl().AddTask()
	defer func() {
		s.Server.Ctrl().DoneTask()
	}()

	return play.CallAction(gctx, s, request)
}

func reload() (int, error) {
	var err error
	var tags []string
	var sockes = make([]*os.File, 0, 1)
	var originalWD, _ = os.Getwd()

	defer func() {
		for _, v := range sockes {
			_ = v.Close()
		}
	}()

	var socketId = 0
	instances.Range(func(key, value interface{}) bool {
		var socket *os.File
		run := value.(runningInstance)
		if run.listener != nil {
			socket, _ = run.listener.(filer).File()
		} else if run.udpListener != nil {
			socket, _ = run.udpListener.(filer).File()
		}
		sockes = append(sockes, socket)
		tags = append(tags, key.(string)+":"+strconv.Itoa(socketId))
		socketId++
		return true
	})

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

	env = append(env, fmt.Sprintf("%s=%s", envGraceful, strings.Join(tags, "-")))
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
			ShutdownAll()
			os.Exit(0)
		case syscall.SIGUSR2:
			if _, err := reload(); err != nil {
				fmt.Println("reload error:", err.Error())
			}
		default:
			fmt.Println("unknown signal")
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

func getGracefulSocket(name string) (id uintptr) {
	if os.Getenv(envGraceful) != "" {
		for _, v := range strings.Split(os.Getenv(envGraceful), "-") {
			if socket := strings.Split(v, ":"); len(socket) == 2 {
				if socket[0] == name {
					socketId, _ := strconv.Atoi(socket[1])
					return uintptr(socketId) + 3
				}
			}
		}
	}
	return
}

type defaultHook struct {
}

func (h defaultHook) OnBoot(server play.IServer) {
}

func (h defaultHook) OnShutdown(server play.IServer) {
}

func (h defaultHook) OnConnect(sess *play.Session, err error) {
	// TODO
}

func (h defaultHook) OnClose(sess *play.Session, err error) {
	// TODO
}

func (h defaultHook) OnRequest(ctx *play.Context) (err error) {
	// TODO
	return
}

func (h defaultHook) OnResponse(ctx *play.Context) (err error) {
	// TODO
	return
}

func (h defaultHook) OnFinish(ctx *play.Context) (err error) {
	// TODO
	return
}
