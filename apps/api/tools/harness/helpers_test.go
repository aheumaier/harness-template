package main

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRoot is the repository the tests run in: the nearest directory above the package that holds
// layout.env, wherever API_DIR puts this tool (README "(b) Brownfield project" step 2).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, layoutFile)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no %s above the test's directory", layoutFile)
		}
		dir = parent
	}
}
