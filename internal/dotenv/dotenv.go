// Package dotenv loads env files the way overmind does, with the same parser,
// so a project's existing files work unchanged.
package dotenv

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/Envek/godotenv"
)

// Files lists the env files for a project rooted at dir, in load order.
// Later files override earlier ones, matching overmind: ~/.overmind.env,
// .overmind.env, .env (unless OVERMIND_SKIP_ENV is set), then each file in
// the comma-separated OVERMIND_ENV.
func Files(dir string, getenv func(string) (string, bool)) []string {
	var files []string
	if home, err := os.UserHomeDir(); err == nil {
		files = append(files, filepath.Join(home, ".overmind.env"))
	}
	files = append(files, filepath.Join(dir, ".overmind.env"))
	if _, skip := getenv("OVERMIND_SKIP_ENV"); !skip {
		files = append(files, filepath.Join(dir, ".env"))
	}
	extra, _ := getenv("OVERMIND_ENV")
	for _, f := range strings.Split(extra, ",") {
		if f == "" {
			continue
		}
		if !filepath.IsAbs(f) {
			f = filepath.Join(dir, f)
		}
		files = append(files, f)
	}
	return files
}

// Load merges the project's env files. A file that fails to parse is skipped
// and reported in warnings rather than stopping the load, as overmind does.
func Load(dir string) (env map[string]string, warnings []error) {
	env = map[string]string{}
	for _, file := range Files(dir, os.LookupEnv) {
		vars, err := godotenv.Read(file)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			warnings = append(warnings, fmt.Errorf("skipping %s: %w", file, err))
		default:
			maps.Copy(env, vars)
		}
	}
	return env, warnings
}
