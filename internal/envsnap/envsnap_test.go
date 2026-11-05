package envsnap

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSaveLoadMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env.json")
	snapshot := []string{"PATH=/asdf/shims:/usr/bin", "REX_BLOCK=start-block", "TERM=old", "SECRET=x"}
	if err := Save(path, snapshot); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", fi.Mode().Perm())
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	got := Merge(loaded, []string{"PATH=/usr/bin", "REX_BLOCK=proc-block", "TERM=rex"})
	want := []string{"PATH=/asdf/shims:/usr/bin", "SECRET=x", "REX_BLOCK=proc-block", "TERM=rex"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
