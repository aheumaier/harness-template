package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
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
	"STACK=go:db:web:ios",
	"IOS_DIR=apps/ios",
}

// noHarnessStack keeps the environment's HARNESS_STACK (a run of make check with a smaller stack) out of a test.
func noHarnessStack(t *testing.T) {
	t.Helper()
	t.Setenv("HARNESS_STACK", "") // restores the value after the test
	os.Unsetenv("HARNESS_STACK")
}

// withLayout is templateLayout with each key, value pair of kv set ("" drops the key).
func withLayout(kv ...string) []string {
	set := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		set[kv[i]] = kv[i+1]
	}
	var out []string
	for _, l := range templateLayout {
		k, _, _ := strings.Cut(l, "=")
		switch v, ok := set[k]; {
		case !ok:
			out = append(out, l)
		case v != "":
			out = append(out, k+"="+v)
		}
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
		"apps/ios/project.yml":                        {"name: App"}, // the iOS app: tests without ios use withoutIOSApp
		"scripts/harness/fixtures/x/files/a/b/c/d.go": {"package d"},
	})
	gitT(t, root, "add", "-A")
	return root
}

// withoutIOSApp removes layoutTree's iOS app, as in a product without ios: its project.yml marks the app (D4).
func withoutIOSApp(t *testing.T, root string) string {
	t.Helper()
	gitT(t, root, "rm", "-qf", "apps/ios/project.yml")
	return root
}

func TestLayoutCheck(t *testing.T) {
	noHarnessStack(t)
	const at = layoutFile + ":"
	tests := []struct {
		name  string
		lines []string
		want  []string // output lines; none means the check passes
	}{
		{"the template's layout passes", templateLayout, nil},
		{"malformed, unknown, repeated and missing keys", append(withLayout("SPEC", ""),
			`SPEC="api/openapi.yaml"`, "LAYOUT_GLOB=x", "API_DIR=apps/api"), []string{
			at + "13: not KEY=value (no spaces, quotes, $ or empty list entries) [harness-layout-valid]",
			at + "14: unknown key LAYOUT_GLOB (known: " + strings.Join(layoutKeys, " ") + ") [harness-layout-valid]",
			at + "15: API_DIR set twice [harness-layout-valid]",
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
		{"a STACK without go, with a repeated and an unknown group", withLayout("STACK", "db:db:ui"), []string{
			at + "12: STACK=db:db:ui: db named twice [harness-layout-valid]",
			at + "12: STACK=db:db:ui: unknown group ui (known: go db web ios) [harness-layout-valid]",
			at + "12: STACK=db:db:ui: no go (every stack has the Go API and the harness) [harness-layout-valid]",
		}},
		{"empty cross-repo lists pass", append(withLayout(), "UPSTREAM_SPECS=", "CONTRACT_CONSUMERS="), nil},
		{"with ios, IOS_DIR holds the iOS app", withLayout("IOS_DIR", "apps/web"), []string{
			at + "13: IOS_DIR=apps/web: no git-known Package.swift or project.yml there, but STACK has ios (set it to the iOS app's directory) [harness-layout-valid]",
		}},
		{"with ios, an IOS_DIR glob is not expanded", withLayout("IOS_DIR", "apps/*"), []string{
			at + "13: IOS_DIR=apps/*: no git-known Package.swift or project.yml there, but STACK has ios (set it to the iOS app's directory) [harness-layout-valid]",
		}},
		{"with ios, IOS_DIR may not be empty", append(withLayout("IOS_DIR", ""), "IOS_DIR="), []string{
			at + "13: IOS_DIR=: no git-known Package.swift or project.yml there, but STACK has ios (set it to the iOS app's directory) [harness-layout-valid]",
		}},
		{"with ios, IOS_DIR is a clean path inside the repo", withLayout("IOS_DIR", "apps/ios/"), []string{
			at + "13: IOS_DIR=apps/ios/: not a clean path inside this repo (no leading /, no ./ or .., no trailing /) [harness-layout-valid]",
		}},
		{"with ios, IOS_DIR is not the repo root", withLayout("IOS_DIR", "."), []string{
			at + "13: IOS_DIR=.: not a clean path inside this repo (no leading /, no ./ or .., no trailing /) [harness-layout-valid]",
		}},
		{"with ios, an IOS_DIR outside the repo is a finding, not a git error", withLayout("IOS_DIR", "../ios"), []string{
			at + "13: IOS_DIR=../ios: not a clean path inside this repo (no leading /, no ./ or .., no trailing /) [harness-layout-valid]",
		}},
		{"with ios, an absolute IOS_DIR is a finding, not a git error", withLayout("IOS_DIR", "/tmp"), []string{
			at + "13: IOS_DIR=/tmp: not a clean path inside this repo (no leading /, no ./ or .., no trailing /) [harness-layout-valid]",
		}},
		{"with ios, IOS_DIR may not be unset", withLayout("IOS_DIR", ""), []string{
			layoutFile + ": IOS_DIR unset, but STACK has ios (set it to the iOS app's directory) [harness-layout-valid]",
		}},
		{"only the cross-repo lists and IOS_DIR may be empty", append(withLayout("SPEC", ""), "SPEC="), []string{
			at + "13: not KEY=value (no spaces, quotes, $ or empty list entries) [harness-layout-valid]",
			layoutFile + ": missing SPEC [harness-layout-valid]",
		}},
		{"UPSTREAM_SPECS entries", append(withLayout(), "UPSTREAM_SPECS=api/openapi.yaml@github.com/acme/up@spec/openapi.yaml@0123abc:api/x.yaml@github.com/acme/up@x.yaml@0123abc:api/openapi.yaml@github.com/acme/up@x.yaml@HEAD:a@b", "CONTRACT_CONSUMERS=github.com/acme/web"), []string{
			at + "14: UPSTREAM_SPECS=api/openapi.yaml@github.com/acme/up@spec/openapi.yaml@0123abc:api/x.yaml@github.com/acme/up@x.yaml@0123abc:api/openapi.yaml@github.com/acme/up@x.yaml@HEAD:a@b: api/x.yaml@github.com/acme/up@x.yaml@0123abc: no such file api/x.yaml (the vendored copy) [harness-layout-valid]",
			at + "14: UPSTREAM_SPECS=api/openapi.yaml@github.com/acme/up@spec/openapi.yaml@0123abc:api/x.yaml@github.com/acme/up@x.yaml@0123abc:api/openapi.yaml@github.com/acme/up@x.yaml@HEAD:a@b: api/openapi.yaml@github.com/acme/up@x.yaml@HEAD: revision HEAD is not a 7 to 40 digit hex commit [harness-layout-valid]",
			at + "14: UPSTREAM_SPECS=api/openapi.yaml@github.com/acme/up@spec/openapi.yaml@0123abc:api/x.yaml@github.com/acme/up@x.yaml@0123abc:api/openapi.yaml@github.com/acme/up@x.yaml@HEAD:a@b: a@b: not <vendored spec>@<source repo>@<spec path in the source>@<revision> [harness-layout-valid]",
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

func TestLayoutStack(t *testing.T) {
	// Paths of the db, web and ios groups that do not exist, as in a product without them.
	lines := withLayout("MIGRATIONS_DIR", "nope", "API_CLIENT_DIR", "nope", "QUERIES_DIR", "nope/queries", "WEB_GLOBS", "web/*", "IOS_DIR", "nope")
	const at = layoutFile + ":"
	for _, tt := range []struct {
		env  string
		want []string
	}{
		{"go", nil},
		{"go,db", []string{
			at + "5: MIGRATIONS_DIR=nope: no such directory [harness-layout-valid]",
			at + "6: QUERIES_DIR=nope/queries: neither it nor its parent directory exists [harness-layout-valid]",
		}},
		{"go:web", []string{
			at + "7: API_CLIENT_DIR=nope: no such directory [harness-layout-valid]",
			at + "8: WEB_GLOBS=web/*: web/* is not <existing directory>/* [harness-layout-valid]",
		}},
		{"go,ios", []string{
			at + "13: IOS_DIR=nope: no git-known Package.swift or project.yml there, but STACK has ios (set it to the iOS app's directory) [harness-layout-valid]",
		}},
		{"", []string{"HARNESS_STACK=: not a list of groups such as go:db:web (HARNESS_STACK may use , instead of :) [harness-layout-valid]"}},
		{"go,web,go", []string{"HARNESS_STACK=go,web,go: go named twice [harness-layout-valid]"}},
	} {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv("HARNESS_STACK", tt.env)
			var out bytes.Buffer
			bad, err := layoutCheck(withoutIOSApp(t, layoutTree(t, lines)), &out)
			want := strings.Join(append(tt.want, ""), "\n")
			if len(tt.want) == 0 {
				want = ""
			}
			if err != nil || bad != (len(tt.want) > 0) || out.String() != want {
				t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), want)
			}
		})
	}
	t.Run("loadLayout sees the override", func(t *testing.T) {
		t.Setenv("HARNESS_STACK", "go,db")
		l, err := loadLayout(layoutTree(t, templateLayout))
		if err != nil || l.get("STACK") != "go:db" || !l.has("db") || l.has("web") {
			t.Errorf("STACK=%q err=%v", l.get("STACK"), err)
		}
	})
}

// TestLayoutStackDefault: a layout.env written before STACK existed (a base ref, an older product) reads as
// go:db:web; a STACK that is present but invalid still fails closed (TestLayoutCheck, TestStackVectors).
func TestLayoutStackDefault(t *testing.T) {
	noHarnessStack(t)
	root := withoutIOSApp(t, layoutTree(t, withLayout("STACK", "")))
	l, err := loadLayout(root)
	if err != nil || l.get("STACK") != "go:db:web" || !l.has("db") || !l.has("web") {
		t.Errorf("STACK=%q err=%v", l.get("STACK"), err)
	}
	var out bytes.Buffer
	if bad, err := layoutCheck(root, &out); bad || err != nil {
		t.Errorf("bad=%v err=%v: %s", bad, err, out.String())
	}
}

// TestLayoutStackHides: a stack that leaves out a group whose git-known files are still in the tree is a
// finding at the STACK line (or HARNESS_STACK), so narrowing the stack never silently drops sensors.
func TestLayoutStackHides(t *testing.T) {
	noHarnessStack(t)
	hides := func(at, group, found string) string {
		return at + ": leaves out " + group + ", but " + found + " exists: add " + group +
			" to STACK or delete its directories (do not narrow HARNESS_STACK on a full tree) [harness-layout-valid]"
	}
	base := map[string][]string{
		"apps/api/go.mod":                   {"module example.com/api"},
		"apps/api/internal/health/api/h.go": {"package api"},
		"apps/api/db/migrations/atlas.sum":  {"h1:"},
		"api/openapi.yaml":                  {"openapi: 3.1.0"},
		"apps/web/README.md":                {"# web"},
		"packages/README.md":                {"# packages"},
		// Fixtures are not the product's code.
		"scripts/harness/fixtures/x/files/apps/api/db/migrations/1.sql": {""},
		"scripts/harness/fixtures/y/files/apps/ios/Sources/A/A.swift":   {""},
	}
	for _, tt := range []struct {
		name  string
		stack string // layout.env's STACK (line 12)
		env   string // HARNESS_STACK, "-" for unset
		add   map[string][]string
		want  []string
	}{
		{"a go-only tree passes", "go", "-", nil, nil},
		{"a migration and a web workspace under STACK=go", "go", "-", map[string][]string{
			"apps/api/db/migrations/1_init.sql": {""},
			"apps/web/package.json":             {"{}"},
		}, []string{
			hides(layoutFile+":12: STACK=go", "db", "apps/api/db/migrations/1_init.sql"),
			hides(layoutFile+":12: STACK=go", "web", "apps/web/package.json"),
		}},
		{"a query file is db code", "go:web", "-", map[string][]string{
			"apps/api/db/queries/users.sql":    {""},
			"packages/api-client/package.json": {"{}"},
		}, []string{hides(layoutFile+":12: STACK=go:web", "db", "apps/api/db/queries/users.sql")}},
		{"API_CLIENT_DIR without a package.json is web code", "go:db", "-", map[string][]string{
			"packages/api-client/src/index.ts": {""},
		}, []string{hides(layoutFile+":12: STACK=go:db", "web", "packages/api-client")}},
		{"Swift under STACK=go", "go", "-", map[string][]string{
			"apps/ios/Sources/AppCore/Greeting.swift": {""},
		}, []string{hides(layoutFile+":12: STACK=go", "ios", "apps/ios/Sources/AppCore/Greeting.swift")}},
		{"an Xcode project is ios code", "go", "-", map[string][]string{
			"apps/mobile/App.xcodeproj/project.pbxproj": {""},
		}, []string{hides(layoutFile+":12: STACK=go", "ios", "apps/mobile/App.xcodeproj/project.pbxproj")}},
		{"an XcodeGen project.yml is ios code, Swift or not", "go", "-", map[string][]string{
			"apps/mobile/project.yml": {"name: App"},
		}, []string{hides(layoutFile+":12: STACK=go", "ios", "apps/mobile/project.yml")}},
		{"a Project.yml is an XcodeGen spec on macOS", "go", "-", map[string][]string{
			"apps/mobile/Project.yml": {"name: App"},
		}, []string{hides(layoutFile+":12: STACK=go", "ios", "apps/mobile/Project.yml")}},
		{"an Xcode project in another case is one on macOS", "go", "-", map[string][]string{
			"apps/mobile/App.XCODEPROJ/Project.pbxproj": {""},
		}, []string{hides(layoutFile+":12: STACK=go", "ios", "apps/mobile/App.XCODEPROJ/Project.pbxproj")}},
		{"Swift in a fixtures/ folder of the app is still the app", "go", "-", map[string][]string{
			"apps/ios/Sources/App/fixtures/Main.swift": {""},
		}, []string{hides(layoutFile+":12: STACK=go", "ios", "apps/ios/Sources/App/fixtures/Main.swift")}},
		{"Swift in a WEB_GLOBS workspace counts, as its tests do", "go", "-", map[string][]string{
			"packages/ios-core/Sources/Core/Core.swift": {""},
		}, []string{hides(layoutFile+":12: STACK=go", "ios", "packages/ios-core/Sources/Core/Core.swift")}},
		{"Swift outside PRODUCT_GLOBS and WEB_GLOBS is not product code", "go", "-", map[string][]string{
			"tools/gen/Main.swift": {""},
		}, nil},
		{"HARNESS_STACK narrows a full tree", "go:db:web:ios", "go", map[string][]string{
			"apps/api/db/migrations/1_init.sql": {""},
			"apps/web/package.json":             {"{}"},
			"packages/api-client/package.json":  {"{}"},
			"apps/ios/Package.swift":            {"// swift-tools-version:6.0"},
		}, []string{
			hides("HARNESS_STACK=go", "db", "apps/api/db/migrations/1_init.sql"),
			hides("HARNESS_STACK=go", "web", "apps/web/package.json"),
			hides("HARNESS_STACK=go", "ios", "apps/ios/Package.swift"),
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := maps.Clone(base)
			maps.Copy(files, tt.add)
			files[layoutFile] = append([]string{"# layout", ""}, withLayout("STACK", tt.stack)...)
			root := linkRepo(t, files)
			gitT(t, root, "add", "-A")
			if tt.env != "-" {
				t.Setenv("HARNESS_STACK", tt.env)
			}
			var out bytes.Buffer
			bad, err := layoutCheck(root, &out)
			want := strings.Join(append(tt.want, ""), "\n")
			if len(tt.want) == 0 {
				want = ""
			}
			if err != nil || bad != (len(tt.want) > 0) || out.String() != want {
				t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), want)
			}
		})
	}
}

// TestLayoutHidesSwiftWhateverTheProductGlobs: a PRODUCT_GLOBS entry is a pathspec with or without a trailing
// `*` (productTests reads both), so the Swift under it is found either way.
func TestLayoutHidesSwiftWhateverTheProductGlobs(t *testing.T) {
	noHarnessStack(t)
	for _, globs := range []string{"apps/ios:api/*", "apps:api"} {
		t.Run(globs, func(t *testing.T) {
			root := linkRepo(t, map[string][]string{
				"apps/api/go.mod":                         {"module example.com/api"},
				"apps/api/internal/health/api/h.go":       {"package api"},
				"api/openapi.yaml":                        {"openapi: 3.1.0"},
				"apps/ios/Sources/AppCore/Greeting.swift": {""},
				layoutFile: append([]string{"# layout", ""}, withLayout("STACK", "go", "PRODUCT_GLOBS", globs)...),
			})
			gitT(t, root, "add", "-A")
			var out bytes.Buffer
			bad, err := layoutCheck(root, &out)
			if err != nil || !bad || !strings.Contains(out.String(), "leaves out ios, but apps/ios/Sources/AppCore/Greeting.swift exists") {
				t.Errorf("bad=%v err=%v, output:\n%s", bad, err, out.String())
			}
		})
	}
}

// TestStackVectors runs the STACK vectors layout.bats shares, so layout.go and layout.sh agree (ADR 0033).
func TestStackVectors(t *testing.T) {
	noHarnessStack(t)
	b, err := os.ReadFile("testdata/stack-vectors.txt")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		value, want, ok := strings.Cut(line, " ")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		n++
		l := layout{values: map[string]string{"STACK": value}, lines: map[string]int{"STACK": 1}}
		if problems := l.applyStack(); (len(problems) == 0) != (want == "accept") {
			t.Errorf("STACK=%s: problems %q; want %s", value, problems, want)
		}
	}
	if n == 0 {
		t.Fatal("no vectors in testdata/stack-vectors.txt")
	}
}

// TestUpstreamSpecVectors runs the UPSTREAM_SPECS entry vectors upstream-drift.bats shares, so layout-check
// refuses what upstream-drift.sh would (ADR 0037).
func TestUpstreamSpecVectors(t *testing.T) {
	b, err := os.ReadFile("testdata/upstream-spec-vectors.txt")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		entry, want, ok := strings.Cut(line, " ")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		n++
		if problem := upstreamEntryProblem(entry); (problem == "") != (want == "ok") {
			t.Errorf("%s: problem %q; want %s", entry, problem, want)
		}
	}
	if n == 0 {
		t.Fatal("no vectors in testdata/upstream-spec-vectors.txt")
	}
}

func TestLayoutRootModule(t *testing.T) {
	noHarnessStack(t)
	// API_DIR=. puts the module at the repo root: paths are joined without a "./" prefix.
	root := linkRepo(t, map[string][]string{
		layoutFile:                   withLayout("API_DIR", "."),
		"go.mod":                     {"module example.com/api"},
		"internal/health/api/h.go":   {"package api"},
		"apps/api/db/migrations/x":   {""},
		"apps/api/db/queries/q.sql":  {""},
		"api/openapi.yaml":           {"openapi: 3.1.0"},
		"packages/api-client/x.json": {"{}"},
		"apps/ios/project.yml":       {"name: App"},
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

// A Swift package alone marks the iOS app (layoutTree has only a project.yml: a .swift file there would be hidden
// Swift under STACK=go).
// So does a project.yml in another case, as macOS reads it.
func TestLayoutCheckIOSPackage(t *testing.T) {
	noHarnessStack(t)
	for _, app := range []string{"apps/pkg/Package.swift", "apps/pkg/Project.yml"} {
		t.Run(app, func(t *testing.T) {
			root := layoutTree(t, withLayout("IOS_DIR", "apps/pkg"))
			writeLines(t, root, app, "// swift-tools-version: 6.0")
			var out bytes.Buffer
			if bad, err := layoutCheck(root, &out); err != nil || bad {
				t.Errorf("bad=%v, err=%v, output:\n%s", bad, err, out.String())
			}
		})
	}
}

func TestLayoutCheckIOSPackageIsARegularFile(t *testing.T) {
	noHarnessStack(t)
	for name, plant := range map[string]func(t *testing.T, root string){
		"a directory named Package.swift": func(t *testing.T, root string) {
			writeLines(t, root, "apps/fake/Package.swift/README", "x")
		},
		"a symlink to /dev/null": func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, "apps/fake"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("/dev/null", filepath.Join(root, "apps/fake/Package.swift")); err != nil {
				t.Fatal(err)
			}
		},
		"a symlink to a regular file": func(t *testing.T, root string) {
			writeLines(t, root, "apps/fake/README", "x")
			if err := os.Symlink("README", filepath.Join(root, "apps/fake/project.yml")); err != nil {
				t.Fatal(err)
			}
		},
		"a git-ignored Package.swift": func(t *testing.T, root string) {
			writeLines(t, root, "apps/fake/Package.swift", "// swift-tools-version: 6.0")
			writeLines(t, root, "apps/fake/.gitignore", "Package.swift")
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := layoutTree(t, withLayout("IOS_DIR", "apps/fake"))
			plant(t, root)
			var out bytes.Buffer
			bad, err := layoutCheck(root, &out)
			if want := "IOS_DIR=apps/fake: no git-known Package.swift or project.yml there"; err != nil || !bad || !strings.Contains(out.String(), want) {
				t.Errorf("bad=%v, err=%v, want %q in output:\n%s", bad, err, want, out.String())
			}
		})
	}
}

func TestLayoutCheckIOSOutsideProductGlobs(t *testing.T) {
	noHarnessStack(t)
	root := layoutTree(t, withLayout("IOS_DIR", "ios"))
	writeLines(t, root, "ios/Package.swift", "// swift-tools-version: 6.0")
	var out bytes.Buffer
	bad, err := layoutCheck(root, &out)
	if want := "IOS_DIR=ios: not under a PRODUCT_GLOBS or WEB_GLOBS entry"; err != nil || !bad || !strings.Contains(out.String(), want) {
		t.Errorf("bad=%v, err=%v, want %q in output:\n%s", bad, err, want, out.String())
	}
}

func TestLayoutCheckExitCodes(t *testing.T) {
	noHarnessStack(t)
	ok := layoutTree(t, templateLayout)
	bad := layoutTree(t, withLayout("LAYER_GLOB", "domains/*/*"))
	for _, tt := range []struct {
		args []string
		want int
	}{
		{[]string{"layout-check", ok}, 0},
		{[]string{"layout-check", bad}, 1},
		{[]string{"layout-check", t.TempDir()}, 2}, // no layout.env
		{[]string{"layout-check"}, exitUsage},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(""), &stdout, &stderr); code != tt.want {
			t.Errorf("%v: exit %d, want %d (%s%s)", tt.args, code, tt.want, stdout.String(), stderr.String())
		}
	}
}

func TestLoadLayoutFailsClosed(t *testing.T) {
	noHarnessStack(t)
	root := linkRepo(t, map[string][]string{layoutFile: withLayout("SPEC", "")})
	if _, err := loadLayout(root); err == nil || err.Error() != layoutFile+": missing SPEC (run make layout-check)" {
		t.Errorf("err = %v", err)
	}
}

func TestLayoutCheckPlatformIsNoDomain(t *testing.T) {
	noHarnessStack(t)
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
		"apps/ios/project.yml":               {"name: App"},
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
