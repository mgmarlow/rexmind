// Package rex is a small client for the Rex server's control API.
package rex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mgmarlow/rexmind/internal/layout"
)

// Caller sends one control-API request and decodes its JSON result into out.
type Caller interface {
	Call(method string, in, out any) error
}

// CLI calls the API through the `rex` command.
type CLI struct{ Bin string }

func (c CLI) bin() string {
	if c.Bin == "" {
		return "rex"
	}
	return c.Bin
}

func (c CLI) Call(method string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return c.run(bytes.NewReader(body), out, "api", "call", method, "-")
}

// Do runs a client action, such as session.select, on the app client.
func (c CLI) Do(action string, args ...string) error {
	return c.run(nil, nil, append([]string{"do", action}, args...)...)
}

func (c CLI) run(stdin *bytes.Reader, out any, args ...string) error {
	cmd := exec.Command(c.bin(), args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rex %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()+" "+err.Error()))
	}
	if out == nil || stdout.Len() == 0 {
		return nil
	}
	return json.Unmarshal(stdout.Bytes(), out)
}

// TerminalFlavor is the block flavor for a terminal.
const TerminalFlavor = "com.superlogical.terminal.shell"

// TerminalOptions are the terminal block options rexmind sets.
type TerminalOptions struct {
	Command []string `json:"command,omitempty"`
	Cwd     string   `json:"cwd,omitempty"`
}

// ResolveSession returns the ID of the session labeled label, or "" if none.
func ResolveSession(c Caller, label string) (string, error) {
	var res struct {
		Results []struct {
			Status string `json:"status"`
			Target struct {
				SessionID string `json:"session_id"`
			} `json:"target"`
		} `json:"results"`
	}
	in := map[string]any{"kinds": []string{"session"}, "queries": []string{label}}
	if err := c.Call("target.resolve", in, &res); err != nil {
		return "", err
	}
	if len(res.Results) == 1 && res.Results[0].Status == "resolved" {
		return res.Results[0].Target.SessionID, nil
	}
	return "", nil
}

// CreateSession creates a session with one window laid out as root and
// returns the session ID and the new block IDs in layout order.
func CreateSession(c Caller, label, window string, root layout.Node) (string, []string, error) {
	var res struct {
		SessionID      string `json:"session_id"`
		InitialWindows []struct {
			BlockIDs []string `json:"block_ids"`
		} `json:"initial_windows"`
	}
	in := map[string]any{
		"label":           label,
		"initial_windows": []any{map[string]any{"window_label": window, "layout": root}},
	}
	if err := c.Call("session.create", in, &res); err != nil {
		return "", nil, err
	}
	if len(res.InitialWindows) != 1 {
		return "", nil, fmt.Errorf("session.create returned %d windows", len(res.InitialWindows))
	}
	return res.SessionID, res.InitialWindows[0].BlockIDs, nil
}

// DestroySession closes every block in the session, hanging up its programs.
func DestroySession(c Caller, sessionID string) error {
	return c.Call("session.destroy", map[string]any{"session_id": sessionID}, nil)
}

// Block is a block in a session.
type Block struct {
	ID    string `json:"block_id"`
	Label string `json:"label"`
}

// ListBlocks returns every block in the session, placed or detached.
func ListBlocks(c Caller, sessionID string) ([]Block, error) {
	var res struct {
		Blocks []Block `json:"blocks"`
	}
	err := c.Call("session.list_blocks", map[string]any{"session_id": sessionID}, &res)
	return res.Blocks, err
}

// Process reports a terminal's root program.
type Process struct {
	Child *struct {
		PID int `json:"pid"`
	} `json:"child"`
}

// RootPID returns the PID of the program a terminal block was started with,
// or 0 if it has exited.
func RootPID(c Caller, sessionID, blockID string) (int, error) {
	var p Process
	if err := blockMethod(c, sessionID, blockID, "process", &p); err != nil {
		return 0, err
	}
	if p.Child == nil {
		return 0, nil
	}
	return p.Child.PID, nil
}

// StatusRecord is one OSC 7501 program status record.
type StatusRecord struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Msg   string `json:"msg"`
}

// ProgramStatus returns the status records programs in the block reported.
func ProgramStatus(c Caller, sessionID, blockID string) ([]StatusRecord, error) {
	var res struct {
		Records []StatusRecord `json:"records"`
	}
	err := blockMethod(c, sessionID, blockID, "program_status", &res)
	return res.Records, err
}

// FocusBlock focuses a block and zooms it to fill its window.
func FocusBlock(c Caller, sessionID, blockID string) error {
	ids := map[string]any{"session_id": sessionID, "block_id": blockID}
	if err := c.Call("session.focus_block", ids, nil); err != nil {
		return err
	}
	ids["zoom"] = true
	return c.Call("session.zoom_block", ids, nil)
}

func blockMethod(c Caller, sessionID, blockID, method string, out any) error {
	in := map[string]any{"session_id": sessionID, "block_id": blockID, "method": method}
	return c.Call("block.method", in, out)
}
