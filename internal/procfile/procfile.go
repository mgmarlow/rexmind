// Package procfile parses Procfile-format process definitions.
package procfile

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Proc is one named command from a Procfile.
type Proc struct {
	Name string
	Cmd  string
}

var line = regexp.MustCompile(`^([A-Za-z0-9_-]+):\s*(.+)$`)

// Parse reads `name: command` lines, skipping blanks and # comments.
func Parse(r io.Reader) ([]Proc, error) {
	var procs []Proc
	seen := map[string]bool{}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		m := line.FindStringSubmatch(text)
		if m == nil {
			return nil, fmt.Errorf("procfile line %d: expected `name: command`", n)
		}
		if seen[m[1]] {
			return nil, fmt.Errorf("procfile line %d: duplicate process %q", n, m[1])
		}
		seen[m[1]] = true
		procs = append(procs, Proc{Name: m[1], Cmd: strings.TrimSpace(m[2])})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(procs) == 0 {
		return nil, fmt.Errorf("procfile defines no processes")
	}
	return procs, nil
}
