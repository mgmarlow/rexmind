package cmd

import (
	"testing"
	"time"
)

func TestDefaultsPreferEnvFilesOverShell(t *testing.T) {
	shell := map[string]string{"OVERMIND_PORT": "4000", "OVERMIND_PORT_STEP": "10"}
	files := map[string]string{"OVERMIND_PORT": "3000", "OVERMIND_NO_PORT": "1", "OVERMIND_TIMEOUT": "15"}
	d := Defaults{Getenv: LayeredEnv(files, func(k string) (string, bool) { v, ok := shell[k]; return v, ok })}

	if got := d.Int("OVERMIND_PORT", 5000); got != 3000 {
		t.Errorf("port = %d, want the env file's 3000", got)
	}
	if got := d.Int("OVERMIND_PORT_STEP", 100); got != 10 {
		t.Errorf("step = %d, want the shell's 10", got)
	}
	if !d.Bool("OVERMIND_NO_PORT") {
		t.Error("no-port should be on")
	}
	if got := d.Seconds("OVERMIND_TIMEOUT", 5*time.Second); got != 15*time.Second {
		t.Errorf("timeout = %v, want 15s", got)
	}
	if d.Err != nil {
		t.Fatal(d.Err)
	}
}

func TestDefaultsReportBadValues(t *testing.T) {
	d := Defaults{Getenv: LayeredEnv(map[string]string{"OVERMIND_PORT": "abc"}, func(string) (string, bool) { return "", false })}
	d.Int("OVERMIND_PORT", 5000)
	if d.Err == nil {
		t.Fatal("want an error for a non-numeric port")
	}
}
