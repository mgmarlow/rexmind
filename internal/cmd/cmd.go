// Package cmd implements rexmind's subcommands.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/mgmarlow/rexmind/internal/dotenv"
	"github.com/mgmarlow/rexmind/internal/envsnap"
	"github.com/mgmarlow/rexmind/internal/layout"
	"github.com/mgmarlow/rexmind/internal/procfile"
	"github.com/mgmarlow/rexmind/internal/project"
	"github.com/mgmarlow/rexmind/internal/rex"
	"github.com/mgmarlow/rexmind/internal/spec"
	"github.com/mgmarlow/rexmind/internal/wrapper"
)

// StartOpts configures `rexmind start`.
type StartOpts struct {
	Spec        spec.Opts
	StopTimeout time.Duration
}

// Start launches every process in a new Rex session, then waits in the
// foreground. Ctrl-C, or the session going away, stops everything.
func Start(c rex.Caller, p project.Project, o StartOpts, out io.Writer) error {
	if _, err := p.Find(c); err == nil {
		return fmt.Errorf("%s is already running; use `rexmind kill` to stop it", p.SessionLabel())
	} else if !errors.Is(err, project.ErrNotRunning) {
		return err
	}
	specs, err := loadSpecs(p, o.Spec)
	if err != nil {
		return err
	}
	_, warnings := dotenv.Load(p.Root, os.LookupEnv)
	for _, w := range warnings {
		fmt.Fprintf(out, "warning: %v\n", w)
	}
	if err := envsnap.Save(envsnap.Path(p.Root), os.Environ()); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	blocks := make([]layout.Block, len(specs))
	for i, s := range specs {
		blocks[i] = layout.Block{
			Flavor:  rex.TerminalFlavor,
			Label:   s.Name,
			Options: rex.TerminalOptions{Command: execArgv(exe, p.Root, s, o.StopTimeout), Cwd: p.Root},
		}
	}
	sessionID, _, err := rex.CreateSession(c, p.SessionLabel(), "procs", layout.Tile(blocks))
	if err != nil {
		os.Remove(envsnap.Path(p.Root))
		return err
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "started %s\n", p.SessionLabel())
	for _, s := range specs {
		fmt.Fprintf(tw, "  %s\tPORT=%d\t%s\n", s.Name, s.Port, s.Cmd)
	}
	fmt.Fprintln(tw, "Ctrl-C stops everything · rexmind connect <name> to view a process")
	tw.Flush()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(interrupt)
	for {
		select {
		case <-interrupt:
			fmt.Fprintln(out, "stopping…")
			return Kill(c, p, o.StopTimeout)
		case <-time.After(2 * time.Second):
			if id, err := rex.ResolveSession(c, p.SessionLabel()); err == nil && id != sessionID {
				fmt.Fprintln(out, "session closed")
				return nil
			}
		}
	}
}

func loadSpecs(p project.Project, o spec.Opts) ([]spec.Spec, error) {
	f, err := os.Open(p.Procfile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	procs, err := procfile.Parse(f)
	if err != nil {
		return nil, err
	}
	return spec.Build(procs, o)
}

func execArgv(exe, root string, s spec.Spec, stop time.Duration) []string {
	return []string{
		exe, "_exec",
		"-root", root,
		"-port", strconv.Itoa(s.Port),
		"-stop-timeout", stop.String(),
		s.Name, "--", "sh", "-c", s.Cmd,
	}
}

// Kill asks every wrapper to stop its process, waits for the blocks to
// close, then destroys the session.
func Kill(c rex.Caller, p project.Project, stopTimeout time.Duration) error {
	r, err := p.Find(c)
	if err != nil {
		return err
	}
	defer os.Remove(envsnap.Path(p.Root))
	if err := signalBlocks(c, r.SessionID, r.Blocks, syscall.SIGTERM); err != nil {
		return err
	}
	deadline := time.Now().Add(stopTimeout + 2*time.Second)
	for time.Now().Before(deadline) {
		blocks, err := rex.ListBlocks(c, r.SessionID)
		if err != nil || len(blocks) == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// The session may already be gone once its last block closed.
	if id, err := rex.ResolveSession(c, p.SessionLabel()); err != nil || id == "" {
		return err
	}
	return rex.DestroySession(c, r.SessionID)
}

// Signal sends sig to the wrappers of the named processes, or of all of them.
func Signal(c rex.Caller, p project.Project, names []string, sig syscall.Signal) error {
	r, err := p.Find(c)
	if err != nil {
		return err
	}
	blocks, err := r.Select(names)
	if err != nil {
		return err
	}
	return signalBlocks(c, r.SessionID, blocks, sig)
}

func signalBlocks(c rex.Caller, sessionID string, blocks []rex.Block, sig syscall.Signal) error {
	var errs []error
	for _, b := range blocks {
		pid, err := rex.RootPID(c, sessionID, b.ID)
		if err == nil && pid > 0 {
			err = syscall.Kill(pid, sig)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b.Label, err))
		}
	}
	return errors.Join(errs...)
}

var stateNames = map[string]string{
	wrapper.Running: "running",
	wrapper.Stopped: "stopped",
	wrapper.Exited:  "exited",
	wrapper.Failed:  "failed",
}

// Status prints each process's last reported state.
func Status(c rex.Caller, p project.Project, out io.Writer) error {
	r, err := p.Find(c)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROCESS\tSTATE\tDETAIL")
	for _, b := range r.Blocks {
		state, msg := "unknown", ""
		records, err := rex.ProgramStatus(c, r.SessionID, b.ID)
		if err != nil {
			msg = err.Error()
		}
		for _, rec := range records {
			if rec.ID == "" {
				state, msg = stateNames[rec.State], rec.Msg
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", b.Label, state, msg)
	}
	return tw.Flush()
}

// Connect shows the process's block in the Rex app, zoomed.
func Connect(c rex.CLI, p project.Project, name string) error {
	r, err := p.Find(c)
	if err != nil {
		return err
	}
	blocks, err := r.Select([]string{name})
	if err != nil {
		return err
	}
	if err := c.Do("session.select", "session_id="+r.SessionID); err != nil {
		return err
	}
	return rex.FocusBlock(c, r.SessionID, blocks[0].ID)
}
