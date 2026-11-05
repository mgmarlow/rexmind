package procfile

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	src := `
# dev processes
web: bin/rails server -p $PORT

worker:   bundle exec sidekiq
`
	got, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []Proc{
		{Name: "web", Cmd: "bin/rails server -p $PORT"},
		{Name: "worker", Cmd: "bundle exec sidekiq"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseRejectsMalformedAndDuplicateLines(t *testing.T) {
	for _, src := range []string{"web bin/server", "web: a\nweb: b", "# only comments"} {
		if _, err := Parse(strings.NewReader(src)); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", src)
		}
	}
}
