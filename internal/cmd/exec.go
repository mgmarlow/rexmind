package cmd

import (
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/mgmarlow/rexmind/internal/dotenv"
	"github.com/mgmarlow/rexmind/internal/envsnap"
	"github.com/mgmarlow/rexmind/internal/wrapper"
)

// ExecOpts configures the wrapper that runs inside a process's block.
type ExecOpts struct {
	Name        string
	Root        string
	Port        int
	StopTimeout time.Duration
	Argv        []string
}

// Exec supervises one process. It is the root program of each block.
func Exec(o ExecOpts) error {
	warn := func(err error) { fmt.Fprintf(os.Stderr, "\x1b[33m[rexmind] %v\x1b[0m\r\n", err) }

	base := os.Environ()
	if snapshot, err := envsnap.Load(envsnap.Path(o.Root)); err == nil {
		base = envsnap.Merge(snapshot, base)
	} else {
		warn(fmt.Errorf("using the block's login environment: %w", err))
	}
	fileEnv, warnings := dotenv.Load(o.Root, lookup(base))
	for _, w := range warnings {
		warn(w)
	}
	port := map[string]string{}
	if o.Port != 0 {
		port["PORT"] = strconv.Itoa(o.Port)
	}
	env := mergeEnv(base, fileEnv, port)

	// A restarted child takes the terminal's foreground from the background,
	// which would otherwise stop it with SIGTTOU.
	tty := isTerminal(os.Stdin)
	if tty {
		signal.Ignore(syscall.SIGTTOU)
	}
	sig := make(chan os.Signal, 4)
	signal.Notify(sig, append([]os.Signal{wrapper.Restart, wrapper.Stop}, wrapper.Shutdown...)...)

	return wrapper.Run(wrapper.Config{
		Argv:        o.Argv,
		Dir:         o.Root,
		Env:         env,
		StopTimeout: o.StopTimeout,
		Foreground:  tty,
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Report:      tee(wrapper.OSCReporter(os.Stdout), announce(os.Stdout, o.Name)),
	}, sig)
}

// mergeEnv layers maps over a KEY=VALUE environment; later layers win.
func mergeEnv(base []string, layers ...map[string]string) []string {
	m := map[string]string{}
	for _, kv := range base {
		if k, v, ok := cutEnv(kv); ok {
			m[k] = v
		}
	}
	for _, l := range layers {
		maps.Copy(m, l)
	}
	out := make([]string, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		out = append(out, k+"="+m[k])
	}
	return out
}

// lookup returns an os.LookupEnv-style function over a KEY=VALUE list.
func lookup(env []string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		for i := len(env) - 1; i >= 0; i-- {
			if k, v, ok := cutEnv(env[i]); ok && k == key {
				return v, true
			}
		}
		return "", false
	}
}

func cutEnv(kv string) (string, string, bool) {
	for i := 1; i < len(kv); i++ {
		if kv[i] == '=' {
			return kv[:i], kv[i+1:], true
		}
	}
	return "", "", false
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func tee(fs ...func(state, msg string)) func(state, msg string) {
	return func(state, msg string) {
		for _, f := range fs {
			f(state, msg)
		}
	}
}

// announce prints a dim line when the process stops running, so the block
// says why it went quiet and how to bring it back.
func announce(w io.Writer, name string) func(state, msg string) {
	return func(state, msg string) {
		if state != wrapper.Running {
			fmt.Fprintf(w, "\r\n\x1b[2m[rexmind] %s %s · rexmind restart %s\x1b[0m\r\n", name, msg, name)
		}
	}
}
