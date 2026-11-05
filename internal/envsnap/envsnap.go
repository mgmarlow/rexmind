// Package envsnap hands the environment `rexmind start` ran with to the
// wrappers in each block. Rex starts blocks with a login-shell environment,
// which misses anything set in interactive startup files such as ~/.zshrc
// (asdf, nvm and pnpm commonly are). Overmind's processes inherit the
// environment `overmind start` ran with, and rexmind keeps that behavior.
package envsnap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Path is where the snapshot for the project at root lives. It holds the
// user's environment, so it sits in the per-user temp dir with mode 0600.
func Path(root string) string {
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(os.TempDir(), "rexmind-"+hex.EncodeToString(sum[:6])+".env.json")
}

// Save writes env (KEY=VALUE entries) to path, readable only by the user.
func Save(path string, env []string) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads a snapshot written by Save.
func Load(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var env []string
	return env, json.Unmarshal(data, &env)
}

// blockOwned reports whether a variable describes the block's own terminal or
// location, so the block's value must win over the snapshot's.
func blockOwned(key string) bool {
	switch key {
	case "TERM", "TERMINFO", "COLORTERM", "PWD", "OLDPWD", "SHLVL":
		return true
	}
	return strings.HasPrefix(key, "REX_") || strings.HasPrefix(key, "TERM_PROGRAM")
}

// Merge returns the snapshot with the block's own terminal and Rex variables
// taking precedence, in KEY=VALUE form.
func Merge(snapshot, block []string) []string {
	out := make([]string, 0, len(snapshot)+8)
	for _, kv := range snapshot {
		if k, _, ok := strings.Cut(kv, "="); ok && !blockOwned(k) {
			out = append(out, kv)
		}
	}
	for _, kv := range block {
		if k, _, ok := strings.Cut(kv, "="); ok && blockOwned(k) {
			out = append(out, kv)
		}
	}
	return out
}
