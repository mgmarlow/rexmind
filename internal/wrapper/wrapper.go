// Package wrapper supervises one Procfile process inside its Rex block.
//
// The wrapper is the block's root program. rexmind controls it by signal:
// SIGUSR2 restarts the child, SIGUSR1 stops it, and SIGHUP, SIGTERM or
// SIGINT stop it and exit. Rex sends SIGHUP when the block or its session
// closes, so hangup always means shutdown.
package wrapper

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Signals the wrapper responds to.
var (
	Restart  = syscall.SIGUSR2
	Stop     = syscall.SIGUSR1
	Shutdown = []os.Signal{syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT}
)

// State values reported for the process, using OSC 7501 state names.
const (
	Running = "working"
	Stopped = "idle"
	Exited  = "done"  // exit status 0
	Failed  = "error" // non-zero exit or killed by a signal
)

// Config describes the process to supervise.
type Config struct {
	Argv        []string
	Dir         string
	Env         []string
	StopTimeout time.Duration
	// Foreground makes each child the terminal's foreground process group,
	// so keyboard input and Ctrl-C reach it. Only valid when stdin is a tty.
	Foreground bool
	Stdout     io.Writer
	Stderr     io.Writer
	Stdin      io.Reader
	// Report is called on every state change.
	Report func(state, msg string)
}

type child struct {
	cmd  *exec.Cmd
	done chan int // receives the exit status once
}

// Run starts the process and handles signals from sig until a shutdown
// signal arrives. It returns after the child has been stopped.
func Run(cfg Config, sig <-chan os.Signal) error {
	c, err := start(cfg)
	if err != nil {
		return err
	}
	for {
		var done <-chan int
		if c != nil {
			done = c.done
		}
		select {
		case code := <-done:
			cfg.Report(exitState(code), fmt.Sprintf("exited %d", code))
			c = nil
		case s := <-sig:
			switch s {
			case Restart:
				stop(cfg, c)
				if c, err = start(cfg); err != nil {
					return err
				}
			case Stop:
				if c != nil {
					stop(cfg, c)
					c = nil
					cfg.Report(Stopped, "stopped")
				}
			default:
				stop(cfg, c)
				return nil
			}
		}
	}
}

func start(cfg Config) (*child, error) {
	if len(cfg.Argv) == 0 {
		return nil, errors.New("no command")
	}
	cmd := exec.Command(cfg.Argv[0], cfg.Argv[1:]...)
	cmd.Dir, cmd.Env = cfg.Dir, cfg.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = cfg.Stdin, cfg.Stdout, cfg.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Foreground: cfg.Foreground}
	if err := cmd.Start(); err != nil {
		cfg.Report(Failed, err.Error())
		return nil, err
	}
	c := &child{cmd: cmd, done: make(chan int, 1)}
	go func() {
		cmd.Wait()
		c.done <- exitCode(cmd.ProcessState)
	}()
	cfg.Report(Running, fmt.Sprintf("running pid %d", cmd.Process.Pid))
	return c, nil
}

// stop sends SIGTERM to the child's process group, then SIGKILL if it is
// still alive after StopTimeout. A nil child is already stopped.
func stop(cfg Config, c *child) {
	if c == nil {
		return
	}
	pgid := -c.cmd.Process.Pid
	syscall.Kill(pgid, syscall.SIGTERM)
	select {
	case <-c.done:
	case <-time.After(cfg.StopTimeout):
		syscall.Kill(pgid, syscall.SIGKILL)
		<-c.done
	}
	// Grandchildren that ignored SIGTERM must not outlive the process.
	syscall.Kill(pgid, syscall.SIGKILL)
}

func exitCode(ps *os.ProcessState) int {
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ps.ExitCode()
}

func exitState(code int) string {
	if code == 0 {
		return Exited
	}
	return Failed
}

// OSCReporter returns a Report func that writes OSC 7501 program status
// sequences to w, so Rex shows the state and `rexmind status` can read it.
func OSCReporter(w io.Writer) func(state, msg string) {
	return func(state, msg string) {
		fmt.Fprintf(w, "\x1b]7501;app=rexmind:state=%s:msg=%s\x1b\\",
			state, base64.StdEncoding.EncodeToString([]byte(msg)))
	}
}
