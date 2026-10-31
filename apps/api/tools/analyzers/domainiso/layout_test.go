package domainiso

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePrefix(t *testing.T) {
	for _, tc := range []struct {
		env, want, err string
	}{
		{"API_DIR=apps/api\nLAYER_GLOB=internal/*/*\n", "/internal/", ""},
		{"# comment\nLAYER_GLOB=pkg/domains/*/*", "/pkg/domains/", ""},
		{"API_DIR=.\n", "", "no LAYER_GLOB"},
		{"LAYER_GLOB=*/*\n", "", "is not"},
		{"LAYER_GLOB=internal/*\n", "", "is not"},
		{"LAYER_GLOB=internal/*/*/x\n", "", "is not"},
		{"LAYER_GLOB=int*/*/*\n", "", "is not"},
	} {
		got, err := parsePrefix([]byte(tc.env))
		if tc.err == "" && (err != nil || got != tc.want) {
			t.Errorf("parsePrefix(%q) = %q, %v; want %q", tc.env, got, err, tc.want)
		}
		if tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)) {
			t.Errorf("parsePrefix(%q) error = %v; want one containing %q", tc.env, err, tc.err)
		}
	}
}

func TestFindPrefixWalksUpAndFailsClosed(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "apps", "api", "internal")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := findPrefix(deep); err == nil || !strings.Contains(err.Error(), "no "+layoutFile) {
		t.Fatalf("findPrefix without layout.env: err = %v; want it to fail closed", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts", "harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(layoutFile)), []byte("LAYER_GLOB=domains/*/*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := findPrefix(deep); err != nil || got != "/domains/" {
		t.Fatalf("findPrefix = %q, %v; want /domains/", got, err)
	}
}

func TestSplitFollowsThePrefix(t *testing.T) {
	for _, tc := range []struct {
		path, prefix, domain, layer string
		ok                          bool
	}{
		{"m/internal/foo/store", "/internal/", "foo", "store", true},
		{"m/domains/foo/api/sub", "/domains/", "foo", "api", true},
		{"m/internal/foo/store", "/domains/", "", "", false},
		{"m/internal/platform/db", "/internal/", "", "", false},
		{"m/internal/foo", "/internal/", "", "", false},
	} {
		d, l, ok := split(tc.path, tc.prefix)
		if d != tc.domain || l != tc.layer || ok != tc.ok {
			t.Errorf("split(%q, %q) = %q, %q, %v; want %q, %q, %v", tc.path, tc.prefix, d, l, ok, tc.domain, tc.layer, tc.ok)
		}
	}
}

// TestParsePrefixVectors runs the LAYER_GLOB vectors layout.go's layerSegments test shares (ADR 0030).
func TestParsePrefixVectors(t *testing.T) {
	b, err := os.ReadFile("../../harness/testdata/layer-globs.txt")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		glob, want, ok := strings.Cut(line, " ")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		n++
		got, err := parsePrefix([]byte("LAYER_GLOB=" + glob + "\n"))
		if want == "reject" && err == nil || want != "reject" && (err != nil || got != want) {
			t.Errorf("parsePrefix(LAYER_GLOB=%s) = %q, %v; want %s", glob, got, err, want)
		}
	}
	if n == 0 {
		t.Fatal("no vectors in tools/harness/testdata/layer-globs.txt")
	}
}
