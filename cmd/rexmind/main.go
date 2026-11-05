// Command rexmind runs Procfile processes in a Rex session.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/mgmarlow/rexmind/internal/cmd"
	"github.com/mgmarlow/rexmind/internal/dotenv"
	"github.com/mgmarlow/rexmind/internal/project"
	"github.com/mgmarlow/rexmind/internal/rex"
	"github.com/mgmarlow/rexmind/internal/spec"
)

const usage = `rexmind runs Procfile processes in a Rex session.

Usage:
  rexmind start   [-f Procfile] [-l web,worker] [-p 5000] [-P 100] [-N] [-t 5s]
  rexmind restart [-f Procfile] [name...]   restart processes (all if none named)
  rexmind stop    [-f Procfile] [name...]   stop processes, keeping their blocks
  rexmind status  [-f Procfile]             show each process's state
  rexmind connect [-f Procfile] name        show a process in the Rex app
  rexmind kill    [-f Procfile]             stop everything and close the session

-t sets how long a process gets after SIGTERM before SIGKILL. It is fixed at
start and applies to every later stop, restart and kill.

start's flags default from overmind's settings, read from the shell or the
project's env files: OVERMIND_PROCFILE, OVERMIND_PROCESSES, OVERMIND_PORT,
OVERMIND_PORT_STEP, OVERMIND_NO_PORT and OVERMIND_TIMEOUT (seconds).
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "rexmind:", err)
		os.Exit(1)
	}
}

func run(name string, args []string) error {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	c := rex.CLI{}

	if name == "_exec" {
		return runExec(fs, args)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// Env file problems are reported by start; here they only cost defaults.
	fileEnv, _ := dotenv.Load(cwd, os.LookupEnv)
	d := cmd.Defaults{Getenv: cmd.LayeredEnv(fileEnv, os.LookupEnv)}

	procfile := fs.String("f", d.String("OVERMIND_PROCFILE", ""), "Procfile path (default Procfile.dev, then Procfile)")
	var only *string
	var portBase, portStep *int
	var noPort *bool
	var stopTimeout *time.Duration
	if name == "start" {
		stopTimeout = fs.Duration("t", d.Seconds("OVERMIND_TIMEOUT", 5*time.Second), "time a process gets after SIGTERM before SIGKILL")
		only = fs.String("l", d.String("OVERMIND_PROCESSES", ""), "comma-separated processes to run (default all)")
		portBase = fs.Int("p", d.Int("OVERMIND_PORT", 5000), "first PORT")
		portStep = fs.Int("P", d.Int("OVERMIND_PORT_STEP", 100), "PORT step between processes")
		noPort = fs.Bool("N", d.Bool("OVERMIND_NO_PORT"), "don't set PORT for processes")
	}
	if d.Err != nil {
		return d.Err
	}
	fs.Parse(args)

	p, err := project.Locate(cwd, *procfile)
	if err != nil {
		return err
	}

	switch name {
	case "start":
		return cmd.Start(c, p, cmd.StartOpts{
			Spec:        spec.Opts{Only: splitList(*only), PortBase: *portBase, PortStep: *portStep, NoPort: *noPort},
			StopTimeout: *stopTimeout,
		}, os.Stdout)
	case "restart":
		return cmd.Signal(c, p, fs.Args(), syscall.SIGUSR2)
	case "stop":
		return cmd.Signal(c, p, fs.Args(), syscall.SIGUSR1)
	case "status":
		return cmd.Status(c, p, os.Stdout)
	case "connect":
		if fs.NArg() != 1 {
			return fmt.Errorf("connect takes one process name")
		}
		return cmd.Connect(c, p, fs.Arg(0))
	case "kill":
		return cmd.Kill(c, p)
	}
	fs.Usage()
	os.Exit(2)
	return nil
}

func runExec(fs *flag.FlagSet, args []string) error {
	root := fs.String("root", "", "project root")
	port := fs.Int("port", 0, "PORT for the process")
	stopTimeout := fs.Duration("stop-timeout", 5*time.Second, "")
	fs.Parse(args)
	if fs.NArg() < 3 || fs.Arg(1) != "--" {
		return fmt.Errorf("usage: rexmind _exec -root DIR -port N NAME -- COMMAND...")
	}
	return cmd.Exec(cmd.ExecOpts{
		Name:        fs.Arg(0),
		Root:        *root,
		Port:        *port,
		StopTimeout: *stopTimeout,
		Argv:        fs.Args()[2:],
	})
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
