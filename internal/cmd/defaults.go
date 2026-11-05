package cmd

import (
	"fmt"
	"strconv"
	"time"
)

// Defaults reads overmind's OVERMIND_* settings, which supply defaults for
// command-line flags. Like overmind, values from env files count as well as
// the shell's, so a project's .overmind.env configures rexmind too.
type Defaults struct {
	Getenv func(string) (string, bool)
	Err    error // the first value that failed to parse
}

// LayeredEnv looks keys up in files first, then in fallback.
func LayeredEnv(files map[string]string, fallback func(string) (string, bool)) func(string) (string, bool) {
	return func(k string) (string, bool) {
		if v, ok := files[k]; ok {
			return v, true
		}
		return fallback(k)
	}
}

func (d *Defaults) String(key, def string) string {
	if v, ok := d.Getenv(key); ok {
		return v
	}
	return def
}

func (d *Defaults) Int(key string, def int) int {
	v, ok := d.Getenv(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	d.fail(key, err)
	return n
}

func (d *Defaults) Bool(key string) bool {
	v, ok := d.Getenv(key)
	if !ok {
		return false
	}
	b, err := strconv.ParseBool(v)
	d.fail(key, err)
	return b
}

// Seconds reads a timeout given in whole seconds, as overmind takes it, or
// as a Go duration such as 10s.
func (d *Defaults) Seconds(key string, def time.Duration) time.Duration {
	v, ok := d.Getenv(key)
	if !ok {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	dur, err := time.ParseDuration(v)
	d.fail(key, err)
	return dur
}

func (d *Defaults) fail(key string, err error) {
	if err != nil && d.Err == nil {
		d.Err = fmt.Errorf("%s: %w", key, err)
	}
}
