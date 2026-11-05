package spec

import (
	"reflect"
	"testing"

	"github.com/mgmarlow/rexmind/internal/procfile"
)

var procs = []procfile.Proc{{Name: "web", Cmd: "w"}, {Name: "worker", Cmd: "k"}, {Name: "css", Cmd: "c"}}

func TestBuildAssignsPortsByPosition(t *testing.T) {
	got, err := Build(procs, Opts{PortBase: 5000, PortStep: 100})
	if err != nil {
		t.Fatal(err)
	}
	want := []Spec{{"web", "w", 5000}, {"worker", "k", 5100}, {"css", "c", 5200}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestBuildFilterKeepsPorts(t *testing.T) {
	got, err := Build(procs, Opts{Only: []string{"css"}, PortBase: 5000, PortStep: 100})
	if err != nil {
		t.Fatal(err)
	}
	if want := []Spec{{"css", "c", 5200}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if _, err := Build(procs, Opts{Only: []string{"nope"}}); err == nil {
		t.Fatal("want error for unknown process")
	}
}
