package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLookPath pins how world.command finds a program: in the world's PATH, never the process's.
func TestLookPath(t *testing.T) {
	tool := func(mode os.FileMode) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "tool"), []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(dir, "tool"), mode); err != nil { // WriteFile's mode is masked by umask
			t.Fatal(err)
		}
		return dir
	}
	runnable, others, notRunnable := tool(0o755), tool(0o071), tool(0o644)
	if os.Geteuid() == 0 { // root may run a file with any execute bit (access(2)), so 0o071 is runnable there
		others = notRunnable
	}
	dirNamedTool := t.TempDir()
	if err := os.Mkdir(filepath.Join(dirNamedTool, "tool"), 0o755); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, tool(0o755))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(runnable, "tool")
	for _, tt := range []struct {
		name    string
		environ []string
	}{
		{"skips a file only group and others may run", []string{"PATH=" + others + ":" + runnable}},
		{"skips a file nobody may run", []string{"PATH=" + notRunnable + ":" + runnable}},
		{"skips a directory", []string{"PATH=" + dirNamedTool + ":" + runnable}},
		{"skips a relative entry (exec.ErrDot)", []string{"PATH=" + relative + ":" + runnable}},
		{"reads the last PATH=, as exec.Cmd passes it on", []string{"PATH=" + others, "PATH=" + runnable}},
	} {
		if got, err := lookPath("tool", tt.environ); got != want || err != nil {
			t.Errorf("%s: %q, %v; want %q", tt.name, got, err, want)
		}
	}
	for _, environ := range [][]string{nil, {"PATH=" + others + ":" + relative}} {
		if _, err := lookPath("tool", environ); !errors.Is(err, exec.ErrNotFound) {
			t.Errorf("PATH %q: %v, want exec.ErrNotFound", environ, err)
		}
	}
}

// TestRunPhysicalCwd: run resolves its working directory, so `..` from a directory reached through a symlink
// names the physical parent, as the OS and a shell's `cd ..; pwd -P` do. It changes the process's directory (only
// run reads it), so it cannot run in parallel.
func TestRunPhysicalCwd(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(dir, "parent")
	if err := os.MkdirAll(filepath.Join(parent, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Join(parent, "child"), link); err != nil {
		t.Fatal(err)
	}
	t.Chdir(link)
	t.Setenv("PWD", link) // os.Getwd returns $PWD when it names the directory, as a shell's cd leaves it
	var stderr bytes.Buffer
	run([]string{"layout-check", ".."}, strings.NewReader(""), io.Discard, &stderr)
	if !strings.Contains(stderr.String(), parent) {
		t.Errorf("layout-check .. from %s: stderr %q; want it to name %s", link, stderr.String(), parent)
	}
}

func TestWorldAbs(t *testing.T) {
	for _, tt := range []struct{ cwd, p, want string }{
		{"/w", "/a/b", "/a/b"},
		{"", "/a/b", "/a/b"},
		{"/w", "a/../b", "/w/b"},
		{"", "a", ""}, // no working directory: an error, never the process's
	} {
		got, err := world{cwd: tt.cwd}.abs(tt.p)
		if got != tt.want || (err != nil) != (tt.want == "") {
			t.Errorf("cwd %q: abs(%q) = %q, %v; want %q", tt.cwd, tt.p, got, err, tt.want)
		}
	}
}
