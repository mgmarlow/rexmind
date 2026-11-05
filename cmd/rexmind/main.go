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
	"github.com/mgmarlow/rexmind/internal/project"
	"github.com/mgmarlow/rexmind/internal/rex"
	"github.com/mgmarlow/rexmind/internal/spec"
)

const usage = `rexmind runs Procfile processes in a Rex session.

Usage:
  rexmind start   [-f Procfile] [-l web,worker] [-p 5000] [-P 100] [-t 5s]
  rexmind restart [-f Procfile] [name...]   restart processes (all if none named)
  rexmind stop    [-f Procfile] [name...]   stop processes, keeping their blocks
  rexmind status  [-f Procfile]             show each process's state
  rexmind connect [-f Procfile] name        show a process in the Rex app
  rexmind kill    [-f Procfile] [-t 5s]     stop everything and close the session
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
	procfile := fs.String("f", "", "Procfile path (default Procfile.dev, then Procfile)")
	stopTimeout := fs.Duration("t", 5*time.Second, "time to wait after SIGTERM before SIGKILL")
	c := rex.CLI{}

	if name == "_exec" {
		return runExec(fs, args)
	}

	var only *string
	var portBase, portStep *int
	if name == "start" {
		only = fs.String("l", "", "comma-separated processes to run (default all)")
		portBase = fs.Int("p", 5000, "first PORT")
		portStep = fs.Int("P", 100, "PORT step between processes")
	}
	fs.Parse(args)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	p, err := project.Locate(cwd, *procfile)
	if err != nil {
		return err
	}

	switch name {
	case "start":
		return cmd.Start(c, p, cmd.StartOpts{
			Spec:        spec.Opts{Only: splitList(*only), PortBase: *portBase, PortStep: *portStep},
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
		return cmd.Kill(c, p, *stopTimeout)
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
