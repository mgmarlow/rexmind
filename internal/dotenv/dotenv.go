// Package dotenv reads .env-style KEY=VALUE files.
package dotenv

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Files are loaded in order; later files override earlier ones.
var Files = []string{".env", ".overmind.env", ".rexmind.env"}

// Parse reads KEY=VALUE lines. It accepts an optional `export ` prefix,
// # comments, and single- or double-quoted values (double quotes expand \n).
func Parse(r io.Reader) (map[string]string, error) {
	env := map[string]string{}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, val, ok := strings.Cut(strings.TrimPrefix(text, "export "), "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", n)
		}
		env[key] = unquote(strings.TrimSpace(val))
	}
	return env, sc.Err()
}

func unquote(v string) string {
	if len(v) >= 2 {
		switch q := v[0]; {
		case q == '\'' && v[len(v)-1] == q:
			return v[1 : len(v)-1]
		case q == '"' && v[len(v)-1] == q:
			return strings.NewReplacer(`\n`, "\n", `\"`, `"`, `\\`, `\`).Replace(v[1 : len(v)-1])
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

// Load merges every file in Files that exists under dir.
func Load(dir string) (map[string]string, error) {
	merged := map[string]string{}
	for _, name := range Files {
		f, err := os.Open(filepath.Join(dir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		env, err := Parse(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for k, v := range env {
			merged[k] = v
		}
	}
	return merged, nil
}
