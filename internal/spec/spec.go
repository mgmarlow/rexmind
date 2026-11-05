// Package spec turns parsed Procfile entries into the processes to run.
package spec

import (
	"fmt"
	"slices"

	"github.com/mgmarlow/rexmind/internal/procfile"
)

// Spec is one process to launch.
type Spec struct {
	Name string
	Cmd  string
	Port int
}

// Opts selects processes and assigns ports.
type Opts struct {
	Only     []string // empty means every process
	PortBase int
	PortStep int
	NoPort   bool // leave PORT unset, so env files or the shell decide it
}

// Build assigns each process PortBase + i*PortStep by its Procfile position,
// so a filtered run gives a process the same port as a full one. With
// NoPort, every Port is 0, meaning PORT is not set.
func Build(procs []procfile.Proc, o Opts) ([]Spec, error) {
	names := make([]string, len(procs))
	for i, p := range procs {
		names[i] = p.Name
	}
	for _, n := range o.Only {
		if !slices.Contains(names, n) {
			return nil, fmt.Errorf("unknown process %q", n)
		}
	}
	var specs []Spec
	for i, p := range procs {
		if len(o.Only) > 0 && !slices.Contains(o.Only, p.Name) {
			continue
		}
		port := o.PortBase + i*o.PortStep
		if o.NoPort {
			port = 0
		}
		specs = append(specs, Spec{Name: p.Name, Cmd: p.Cmd, Port: port})
	}
	return specs, nil
}
