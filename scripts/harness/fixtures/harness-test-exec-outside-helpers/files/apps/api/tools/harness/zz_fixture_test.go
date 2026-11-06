package main

import (
	proc "os/exec"
	"testing"
)

func TestFixtureExec(t *testing.T) {
	t.Parallel()
	if err := proc.Command("true").Run(); err != nil {
		t.Fatal(err)
	}
}
