// Package project ties a Procfile on disk to the Rex session running it.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mgmarlow/rexmind/internal/rex"
)

// DefaultProcfiles are tried in order when no Procfile is named.
var DefaultProcfiles = []string{"Procfile.dev", "Procfile"}

// Project is a Procfile and the directory its processes run in.
type Project struct {
	Root     string
	Procfile string
}

// Locate finds the Procfile: procfile if given (relative to dir), otherwise
// the first of DefaultProcfiles in dir. Root is the Procfile's directory.
func Locate(dir, procfile string) (Project, error) {
	candidates := DefaultProcfiles
	if procfile != "" {
		candidates = []string{procfile}
	}
	for _, name := range candidates {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, name)
		}
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return Project{}, err
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return Project{}, err
		}
		return Project{Root: filepath.Dir(abs), Procfile: abs}, nil
	}
	return Project{}, fmt.Errorf("no Procfile found in %s (tried %v)", dir, candidates)
}

// SessionLabel names the Rex session that runs the project.
func (p Project) SessionLabel() string {
	return "rexmind:" + filepath.Base(p.Root)
}

// Running is a project's live Rex session.
type Running struct {
	SessionID string
	Blocks    []rex.Block
}

// ErrNotRunning means no Rex session runs the project.
var ErrNotRunning = errors.New("not running")

// Find returns the project's running session and its process blocks.
func (p Project) Find(c rex.Caller) (Running, error) {
	id, err := rex.ResolveSession(c, p.SessionLabel())
	if err != nil {
		return Running{}, err
	}
	if id == "" {
		return Running{}, fmt.Errorf("%s: %w", p.SessionLabel(), ErrNotRunning)
	}
	blocks, err := rex.ListBlocks(c, id)
	return Running{SessionID: id, Blocks: blocks}, err
}

// Select returns the blocks labeled with names, or every block if names is
// empty. An unknown name is an error.
func (r Running) Select(names []string) ([]rex.Block, error) {
	if len(names) == 0 {
		return r.Blocks, nil
	}
	byLabel := map[string]rex.Block{}
	for _, b := range r.Blocks {
		byLabel[b.Label] = b
	}
	picked := make([]rex.Block, 0, len(names))
	for _, n := range names {
		b, ok := byLabel[n]
		if !ok {
			return nil, fmt.Errorf("no running process %q", n)
		}
		picked = append(picked, b)
	}
	return picked, nil
}
