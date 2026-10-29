package main

import (
	"bytes"
	"maps"
	"os"
	"strings"
	"testing"
)

// templateLayout is the template's scripts/harness/layout.env, without comments.
var templateLayout = []string{
	"API_DIR=apps/api",
	"SPEC=api/openapi.yaml",
	"MIGRATIONS_DIR=apps/api/db/migrations",
	"QUERIES_DIR=apps/api/db/queries",
	"API_CLIENT_DIR=packages/api-client",
	"WEB_GLOBS=apps/*:packages/*",
	"GENERATED_GLOBS=*.gen.go:apps/api/internal/*/store/*:packages/api-client/src/gen/*:docs/generated/*",
	"PRODUCT_GLOBS=apps/*:api/*",
	"LAYER_GLOB=internal/*/*",
}

// withLayout is templateLayout with key set to value ("" drops the key).
func withLayout(key, value string) []string {
	var out []string
	for _, l := range templateLayout {
		if strings.HasPrefix(l, key+"=") {
			if value != "" {
				out = append(out, key+"="+value)
			}
			continue
		}
		out = append(out, l)
	}
	return out
}

// layoutTree is a repo whose tree matches templateLayout, with layout.env holding lines.
func layoutTree(t *testing.T, lines []string) string {
	t.Helper()
	root := linkRepo(t, map[string][]string{
		layoutFile:                                    append([]string{"# layout", ""}, lines...),
		"apps/api/go.mod":                             {"module example.com/api"},
		"apps/api/internal/health/api/h.go":           {"package api"},
		"apps/api/db/migrations/atlas.sum":            {"h1:"},
		"api/openapi.yaml":                            {"openapi: 3.1.0"},
		"packages/api-client/package.json":            {"{}"},
		"scripts/harness/fixtures/x/files/a/b/c/d.go": {"package d"},
	})
	gitT(t, root, "add", "-A")
	return root
}

func TestLayoutCheck(t *testing.T) {
	const at = layoutFile + ":"
	tests := []struct {
		name  string
		lines []string
		want  []string // output lines; none means the check passes
	}{
		{"the template's layout passes", templateLayout, nil},
		{"malformed, unknown, repeated and missing keys", append(withLayout("SPEC", ""),
			`SPEC="api/openapi.yaml"`, "LAYOUT_GLOB=x", "API_DIR=apps/api"), []string{
			at + "11: not KEY=value (no spaces, quotes, $ or empty list entries) [harness-layout-valid]",
			at + "12: unknown key LAYOUT_GLOB (known: " + strings.Join(layoutKeys, " ") + ") [harness-layout-valid]",
			at + "13: API_DIR set twice [harness-layout-valid]",
			layoutFile + ": missing SPEC [harness-layout-valid]",
		}},
		{"a list with an empty entry", withLayout("PRODUCT_GLOBS", "apps/*:"), []string{
			at + "10: not KEY=value (no spaces, quotes, $ or empty list entries) [harness-layout-valid]",
			layoutFile + ": missing PRODUCT_GLOBS [harness-layout-valid]",
		}},
		{"paths that do not exist", withLayout("SPEC", "api/missing.yaml"), []string{
			at + "4: SPEC=api/missing.yaml: no such file (the OpenAPI contract) [harness-layout-valid]",
		}},
		{"API_DIR without a go.mod", withLayout("API_DIR", "apps"), []string{
			at + "3: API_DIR=apps: no go.mod there (the Go API module; `.` for the repo root) [harness-layout-valid]",
			at + "11: LAYER_GLOB=internal/*/*: matches no package (no directory apps/internal/*/* with a .go file): the domain-isolation analyzer would check nothing [harness-layout-valid]",
		}},
		{"a directory that does not exist", withLayout("MIGRATIONS_DIR", "nope"), []string{
			at + "5: MIGRATIONS_DIR=nope: no such directory [harness-layout-valid]",
		}},
		{"QUERIES_DIR may be missing until the first domain, its parent may not", withLayout("QUERIES_DIR", "nope/queries"), []string{
			at + "6: QUERIES_DIR=nope/queries: neither it nor its parent directory exists [harness-layout-valid]",
		}},
		{"WEB_GLOBS entries are <existing dir>/*", withLayout("WEB_GLOBS", "apps/*:web/*:apps"), []string{
			at + "8: WEB_GLOBS=apps/*:web/*:apps: web/* is not <existing directory>/* [harness-layout-valid]",
			at + "8: WEB_GLOBS=apps/*:web/*:apps: apps is not <existing directory>/* [harness-layout-valid]",
		}},
		{"a LAYER_GLOB that is not <dir>/*/*", withLayout("LAYER_GLOB", "internal/*"), []string{
			at + "11: LAYER_GLOB=internal/*: not <dir>[/<dir>…]/*/* (literal directories, then the domain and the layer) [harness-layout-valid]",
		}},
		{"a LAYER_GLOB with a wildcard directory", withLayout("LAYER_GLOB", "int*/*/*"), []string{
			at + "11: LAYER_GLOB=int*/*/*: segment \"int*\" is not a literal directory name [harness-layout-valid]",
		}},
		{"a LAYER_GLOB that matches no package (fixtures do not count)", withLayout("LAYER_GLOB", "domains/*/*"), []string{
			at + "11: LAYER_GLOB=domains/*/*: matches no package (no directory apps/api/domains/*/* with a .go file): the domain-isolation analyzer would check nothing [harness-layout-valid]",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			bad, err := layoutCheck(layoutTree(t, tt.lines), &out)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Join(tt.want, "\n")
			if len(tt.want) > 0 {
				want += "\n"
			}
			if bad != (len(tt.want) > 0) || out.String() != want {
				t.Errorf("bad=%v, output:\n%s\nwant:\n%s", bad, out.String(), want)
			}
		})
	}
}

func TestLayoutRootModule(t *testing.T) {
	// API_DIR=. puts the module at the repo root: paths are joined without a "./" prefix.
	root := linkRepo(t, map[string][]string{
		layoutFile:                   withLayout("API_DIR", "."),
		"go.mod":                     {"module example.com/api"},
		"internal/health/api/h.go":   {"package api"},
		"apps/api/db/migrations/x":   {""},
		"apps/api/db/queries/q.sql":  {""},
		"api/openapi.yaml":           {"openapi: 3.1.0"},
		"packages/api-client/x.json": {"{}"},
	})
	gitT(t, root, "add", "-A")
	l, err := loadLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := l.apiPath("go.mod"); got != "go.mod" {
		t.Errorf("apiPath(go.mod) = %q, want go.mod", got)
	}
	var out bytes.Buffer
	if bad, err := layoutCheck(root, &out); bad || err != nil {
		t.Errorf("bad=%v err=%v: %s", bad, err, out.String())
	}
}

func TestLayoutCheckExitCodes(t *testing.T) {
	ok := layoutTree(t, templateLayout)
	bad := layoutTree(t, withLayout("LAYER_GLOB", "domains/*/*"))
	for _, tt := range []struct {
		args []string
		want int
	}{
		{[]string{"layout-check", ok}, 0},
		{[]string{"layout-check", bad}, 1},
		{[]string{"layout-check", t.TempDir()}, 2}, // no layout.env
		{[]string{"layout-check"}, 2},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(""), &stdout, &stderr); code != tt.want {
			t.Errorf("%v: exit %d, want %d (%s%s)", tt.args, code, tt.want, stdout.String(), stderr.String())
		}
	}
}

func TestLoadLayoutFailsClosed(t *testing.T) {
	root := linkRepo(t, map[string][]string{layoutFile: withLayout("SPEC", "")})
	if _, err := loadLayout(root); err == nil || err.Error() != layoutFile+": missing SPEC (run make layout-check)" {
		t.Errorf("err = %v", err)
	}
}

func TestLayoutCheckPlatformIsNoDomain(t *testing.T) {
	// domainiso skips <layer dir>/platform and every flat <layer dir>/<pkg>. A platform-only tree is the
	// template before its first domain, so it passes; a flat package beside it is a layout domainiso cannot see.
	files := map[string][]string{
		layoutFile:                           templateLayout,
		"apps/api/go.mod":                    {"module example.com/api"},
		"apps/api/internal/platform/db/d.go": {"package db"},
		"apps/api/internal/platform/p.go":    {"package platform"},
		"apps/api/db/migrations/atlas.sum":   {"h1:"},
		"api/openapi.yaml":                   {"openapi: 3.1.0"},
		"packages/api-client/package.json":   {"{}"},
	}
	for _, tc := range []struct {
		name string
		add  map[string][]string
		want string
	}{
		{"platform only passes", nil, ""},
		{"a flat package fails", map[string][]string{"apps/api/internal/billing/b.go": {"package billing"}},
			layoutFile + ":9: LAYER_GLOB=internal/*/*: apps/api/internal/billing is a package with no layer (expected apps/api/internal/<domain>/<layer>): the domain-isolation analyzer skips it [harness-layout-valid]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			all := maps.Clone(files)
			maps.Copy(all, tc.add)
			root := linkRepo(t, all)
			gitT(t, root, "add", "-A")
			var out bytes.Buffer
			bad, err := layoutCheck(root, &out)
			if err != nil || bad != (tc.want != "") || out.String() != tc.want {
				t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), tc.want)
			}
		})
	}
}

// TestLayerSegmentsVectors runs the LAYER_GLOB vectors domainiso's parsePrefix test shares (ADR 0030).
func TestLayerSegmentsVectors(t *testing.T) {
	b, err := os.ReadFile("testdata/layer-globs.txt")
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
		if _, err := layerSegments(glob); (err == nil) != (want != "reject") {
			t.Errorf("layerSegments(%q) error = %v; want %s", glob, err, want)
		}
	}
	if n == 0 {
		t.Fatal("no vectors in testdata/layer-globs.txt")
	}
}
