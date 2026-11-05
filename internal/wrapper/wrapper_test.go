package wrapper

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type report struct{ state, msg string }

// harness runs a wrapper around argv and exposes its reports and signals.
type harness struct {
	t       *testing.T
	sig     chan os.Signal
	reports chan report
	done    chan error
}

func run(t *testing.T, timeout time.Duration, argv ...string) *harness {
	h := &harness{t: t, sig: make(chan os.Signal, 1), reports: make(chan report, 16), done: make(chan error, 1)}
	cfg := Config{
		Argv:        argv,
		StopTimeout: timeout,
		Report:      func(state, msg string) { h.reports <- report{state, msg} },
	}
	go func() { h.done <- Run(cfg, h.sig) }()
	return h
}

func (h *harness) next() report {
	h.t.Helper()
	select {
	case r := <-h.reports:
		return r
	case <-time.After(5 * time.Second):
		h.t.Fatal("timed out waiting for a report")
		return report{}
	}
}

func (h *harness) shutdown() {
	h.t.Helper()
	h.sig <- syscall.SIGTERM
	select {
	case err := <-h.done:
		if err != nil {
			h.t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		h.t.Fatal("wrapper did not exit")
	}
}

func TestRestartStopAndShutdown(t *testing.T) {
	h := run(t, time.Second, "sleep", "30")

	first := h.next()
	if first.state != Running {
		t.Fatalf("first report = %+v", first)
	}
	h.sig <- Restart
	if r := h.next(); r.state != Running || r.msg == first.msg {
		t.Fatalf("after restart = %+v, want a new running pid", r)
	}
	h.sig <- Stop
	if r := h.next(); r.state != Stopped {
		t.Fatalf("after stop = %+v", r)
	}
	h.sig <- Restart
	if r := h.next(); r.state != Running {
		t.Fatalf("restart from stopped = %+v", r)
	}
	h.shutdown()
}

func TestReportsExitStatusAndStaysUp(t *testing.T) {
	h := run(t, time.Second, "sh", "-c", "exit 3")
	h.next()
	if r := h.next(); r != (report{Failed, "exited 3"}) {
		t.Fatalf("got %+v", r)
	}
	h.shutdown()
}

// waitFor polls until path exists.
func waitFor(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never appeared", path)
}

func TestStopGivesGroupMembersTheGracePeriod(t *testing.T) {
	dir := t.TempDir()
	ready, cleaned := filepath.Join(dir, "ready"), filepath.Join(dir, "cleaned")
	worker := `trap "sleep 0.2; touch ` + cleaned + `; exit 0" TERM; touch ` + ready + `; while :; do sleep 0.05; done`
	h := run(t, 2*time.Second, "sh", "-c", "sh -c '"+worker+"' & wait")
	h.next()
	waitFor(t, ready)

	h.shutdown()
	if _, err := os.Stat(cleaned); err != nil {
		t.Fatal("worker was killed before finishing its cleanup")
	}
}

func TestExitStopsLeftoverBackgroundJobs(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "pid")
	h := run(t, time.Second, "sh", "-c", "sleep 300 & echo $! > "+pidfile)
	h.next()
	if r := h.next(); r.state != Exited {
		t.Fatalf("got %+v", r)
	}
	data, _ := os.ReadFile(pidfile)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid == 0 || syscall.Kill(pid, 0) == nil {
		t.Fatalf("background job %d still alive", pid)
	}
	h.shutdown()
}

func TestStopEscalatesToKill(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	h := run(t, 100*time.Millisecond, "sh", "-c", "trap '' TERM; touch "+ready+"; while :; do sleep 0.05; done")
	pid, _ := strconv.Atoi(strings.TrimPrefix(h.next().msg, "running pid "))
	for _, err := os.Stat(ready); err != nil; _, err = os.Stat(ready) {
		time.Sleep(10 * time.Millisecond)
	}
	start := time.Now()
	h.shutdown()
	if time.Since(start) < 100*time.Millisecond {
		t.Fatal("shutdown returned before the stop timeout")
	}
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatalf("process %d still alive", pid)
	}
}
