package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mgmarlow/rexmind/internal/rex"
)

// fakeRex answers control-API calls with canned JSON by method name.
type fakeRex map[string]string

func (f fakeRex) Call(method string, _, out any) error {
	body, ok := f[method]
	if !ok {
		return errors.New("unexpected call " + method)
	}
	return json.Unmarshal([]byte(body), out)
}

func TestLocatePrefersProcfileDev(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Procfile"), nil, 0o600)
	os.WriteFile(filepath.Join(dir, "Procfile.dev"), nil, 0o600)
	p, err := Locate(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p.Procfile) != "Procfile.dev" || p.SessionLabel() != "rexmind:"+filepath.Base(dir) {
		t.Fatalf("got %+v label %s", p, p.SessionLabel())
	}
}

func TestFindAndSelect(t *testing.T) {
	c := fakeRex{
		"target.resolve":      `{"results":[{"status":"resolved","target":{"session_id":"s1"}}]}`,
		"session.list_blocks": `{"blocks":[{"block_id":"b1","label":"web"},{"block_id":"b2","label":"worker"}]}`,
	}
	r, err := Project{Root: "/x/app"}.Find(c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Select([]string{"worker"})
	if err != nil || len(got) != 1 || got[0] != (rex.Block{ID: "b2", Label: "worker"}) {
		t.Fatalf("Select = %+v, %v", got, err)
	}
	if _, err := r.Select([]string{"nope"}); err == nil {
		t.Fatal("want error for unknown process")
	}
}

func TestFindNotRunning(t *testing.T) {
	c := fakeRex{"target.resolve": `{"results":[{"status":"not_found"}]}`}
	if _, err := (Project{Root: "/x/app"}).Find(c); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("err = %v, want ErrNotRunning", err)
	}
}
