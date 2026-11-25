package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"
)

// stopLimit is what one stop may cost (ADR 0048). Past it, the check's processes get killGrace to end and the
// second key afterLimit; the hook's timeout in .claude/settings.json is longer than all of it, or Claude Code would
// kill the gate and let the stop through.
const (
	stopLimit  = 15 * time.Second
	afterLimit = time.Second
)

const stopRule = "harness-stop-is-green"

func init() {
	commands.register(hookGuard("stop-gate", stopRule+": the Claude Code Stop hook, its JSON on stdin (.claude/hooks/stop-gate.sh)",
		func(w world, root string) error {
			in, err := parseStopInput(w.stdin)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
			defer stop()
			return stopGate(ctx, w, root, in, stopLimit, afterLimit)
		}))
}

// stopInput is the part of Claude Code's Stop input the gate reads.
type stopInput struct {
	PermissionMode  string `json:"permission_mode"`
	StopHookActive  *bool  `json:"stop_hook_active"`
	BackgroundTasks []struct {
		Type    string `json:"type"`
		Status  string `json:"status"`
		Command string `json:"command"`
	} `json:"background_tasks"`
}

// repeated reports a stop that follows a block. An input that does not say counts as one, so that the gate can
// never block twice in a row.
func (in stopInput) repeated() bool { return in.StopHookActive == nil || *in.StopHookActive }

// busy reports background work whose end fires the hook again: a subagent, whose tree is half-written, or a shell
// that runs the check and nothing else.
func (in stopInput) busy() bool {
	for _, t := range in.BackgroundTasks {
		if t.Status == "running" && (t.Type == "subagent" || t.Type == "shell" && checkCommand.MatchString(t.Command)) {
			return true
		}
	}
	return false
}

// checkCommand matches `make check` or `make check-all` with a cd or assignments before it and options, assignments,
// redirections and pipes after it. `make check && make dev` is not one: it would silence the gate for as long as it
// runs. `make check | sleep 99` is, and does (ADR 0048). A check it misses only costs the gate's own.
var checkCommand = regexp.MustCompile(`^(cd [^\s;&|]+ && )?(\w+=[^\s;&|]* )*make( -[^\s;&|]+)* check(-all)?` +
	`( (-|\w+=)[^\s;&|]*)*( ?\d?(>&|>|\|[^;&|\n>])[^;&|\n>]*)*$`)

// parseStopInput is not parseHookInput: `null` is an input that does not say which stop it is, not an error.
func parseStopInput(r io.Reader) (in stopInput, err error) {
	b, err := io.ReadAll(r)
	if err == nil {
		err = json.Unmarshal(b, &in)
	}
	if err != nil {
		err = fmt.Errorf("cannot parse the hook input: %w", err)
	}
	return in, err
}

// stopGate returns an error to hold the session back, once per turn. limit covers the key and the check, after the
// second key (stopLimit and afterLimit; tests take others).
func stopGate(ctx context.Context, w world, root string, in stopInput, limit, after time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	w.ctx = ctx
	if in.PermissionMode == "plan" {
		return nil // a plan session changes nothing, and must not run the tree's code
	}
	key, keyErr := w.treeKey(root)
	switch {
	case keyErr == nil && (key.stamped(root) || w.isBase(root, key)):
		return nil
	case in.repeated():
		msg := stopRule + ": " + root + " is not known to be green; make check did not run again at this stop"
		if keyErr != nil {
			msg += fmt.Sprintf(" (cannot tell which tree this is: %v)", keyErr)
		}
		return json.NewEncoder(w.stdout).Encode(map[string]string{"systemMessage": msg})
	case in.busy():
		return nil
	}
	started := ctx.Err() == nil
	out, err := w.check(root)
	var red *exec.ExitError
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil: // before red: a check that was ended has an exit status too
		why := fmt.Sprintf("was not done after %s", limit)
		if errors.Is(ctx.Err(), context.Canceled) {
			why = "was ended by a signal"
		}
		hint := ""
		if started && w.changed(root, key, after) {
			hint = " The check that was cut off may have left generated files half-written: run `make gen` first."
		}
		return blockf("[%s] `make check` %s in %s, so this tree is not checked. To proceed: run `make check` "+
			"yourself, fix what you changed, then give your final answer again.%s", stopRule, why, root, hint)
	case errors.As(err, &red):
		return blockf("[%s] `make check` is red in %s. To proceed: fix what you changed and leave what is not yours, "+
			"run `make check`, then give your final answer again.\n%s", stopRule, root, digest(out))
	}
	return fmt.Errorf("[%s] cannot run `make check` in %s: %w", stopRule, root, err)
}

// changed reports whether root's key is no longer k; so is one that takes longer than limit.
func (w world) changed(root string, k treeKey, limit time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	w.ctx = ctx
	now, err := w.treeKey(root)
	return err != nil || now != k
}

// isBase reports whether k's tree is the one of its merge base: nothing changed.
func (w world) isBase(root string, k treeKey) bool {
	if k.base == noBase {
		return false
	}
	w.stderr = io.Discard
	out, err := w.git(root, "rev-parse", "-q", "--verify", k.base+"^{tree}")
	return err == nil && strings.TrimSpace(string(out)) == k.tree
}

// check runs `make check` in root and returns its output. A caller's make variables (`make -i`, `-n`, another
// makefile, another make) are not passed on: they would make a red check end green.
func (w world) check(root string) ([]byte, error) {
	// A file, not a pipe: a process that outlives make would keep a pipe open and Wait waiting.
	f, err := os.CreateTemp(w.getenv("TMPDIR"), "harness-stop-gate-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	w.environ = slices.DeleteFunc(slices.Clone(w.environ), func(kv string) bool {
		k, _, _ := strings.Cut(kv, "=")
		return strings.HasPrefix(k, "MAKE") || k == "MFLAGS" || k == "GNUMAKEFLAGS"
	})
	cmd := w.command("make", "check")
	cmd.Dir, cmd.Stdout, cmd.Stderr = root, f, f
	if err := cmd.Run(); err != nil {
		out, _ := os.ReadFile(f.Name())
		return out, err
	}
	return nil, nil
}

// digest is what a red check says: every failed sensor with its rules and their remediation, then the end of the output.
func digest(out []byte) string {
	const tail = 40
	lines := strings.Split(strings.TrimRight(string(bytes.ToValidUTF8(out, nil)), "\n"), "\n")
	start := max(0, len(lines)-tail)
	var b strings.Builder
	for _, l := range lines[:start] {
		if strings.HasPrefix(l, "✗ sensor ") || strings.HasPrefix(l, "rule: ") || strings.HasPrefix(l, "remediation: ") {
			fmt.Fprintln(&b, l)
		}
	}
	if start > 0 {
		fmt.Fprintf(&b, "… the last %d of %d lines:\n", tail, len(lines))
	}
	b.WriteString(strings.Join(lines[start:], "\n"))
	return b.String()
}
