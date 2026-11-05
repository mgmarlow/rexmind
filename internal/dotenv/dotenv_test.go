package dotenv

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	src := `
# comment
PLAIN=value
export EXPORTED=yes
SINGLE='a $literal'
DOUBLE="line1\nline2"
TRAILING=x # note
`
	got, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"PLAIN":    "value",
		"EXPORTED": "yes",
		"SINGLE":   "a $literal",
		"DOUBLE":   "line1\nline2",
		"TRAILING": "x",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoadLaterFilesOverride(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("A=1\nB=1\n"), 0o600)
	os.WriteFile(filepath.Join(dir, ".overmind.env"), []byte("B=2\n"), 0o600)
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"A": "1", "B": "2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
