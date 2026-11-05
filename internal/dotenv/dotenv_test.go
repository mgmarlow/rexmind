package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergesInOvermindOrder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	os.WriteFile(filepath.Join(dir, ".overmind.env"), []byte("A=overmind\nB=overmind\n"), 0o600)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("B=dotenv\nKEY=\"multi\nline\"\n"), 0o600)

	env, warnings := Load(dir, os.LookupEnv)
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	want := map[string]string{"A": "overmind", "B": "dotenv", "KEY": "multi\nline"}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
}

func TestLoadSkipsUnparsableFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	os.WriteFile(filepath.Join(dir, ".overmind.env"), []byte("A=1\n"), 0o600)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("not an assignment\n"), 0o600)

	env, warnings := Load(dir, os.LookupEnv)
	if len(warnings) != 1 || env["A"] != "1" {
		t.Fatalf("env = %v, warnings = %v", env, warnings)
	}
}

func TestFilesHonorsOvermindEnvVars(t *testing.T) {
	vars := map[string]string{"OVERMIND_SKIP_ENV": "1", "OVERMIND_ENV": "a.env,/abs/b.env"}
	getenv := func(k string) (string, bool) { v, ok := vars[k]; return v, ok }

	files := Files("/proj", getenv)
	tail := files[len(files)-3:]
	want := []string{"/proj/.overmind.env", "/proj/a.env", "/abs/b.env"}
	for i := range want {
		if tail[i] != want[i] {
			t.Fatalf("files = %v, want suffix %v", files, want)
		}
	}
}
