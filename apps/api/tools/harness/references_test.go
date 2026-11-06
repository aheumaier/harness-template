package main

import (
	"bytes"
	"strings"
	"testing"
)

// refHeader is a reference file header as docs/references/*-llms.txt carry it: Version on line 5, Pin on
// line 6 (or 5 without Version), Sources after them.
func refHeader(version, pin string, sources ...string) []string {
	lines := []string{"# tool", "", "> summary", ""}
	if version != "" {
		lines = append(lines, "Version: "+version)
	}
	if pin != "" {
		lines = append(lines, "Pin: "+pin)
	}
	for _, s := range sources {
		lines = append(lines, "Source: "+s)
	}
	for len(lines) < headerLines {
		lines = append(lines, "")
	}
	return append(lines, "Version: v0.0.0 below the header window does not count")
}

const (
	toolEnv = "scripts/tool-versions.env"
	toolPin = toolEnv + " TOOL_VERSION"
	toolURL = "https://example.com/tool/blob/v1.2.3/README.md"
	toolRef = "docs/references/tool-llms.txt"
)

func TestReferencesPinned(t *testing.T) {
	t.Parallel()
	env := []string{"# pins", "", "TOOL_VERSION=v1.2.3", "BARE_VERSION=1.3.0"}
	pkg := []string{`{"dependencies": {"lib": "2.0.0"}, "devDependencies": {"gen": "8.45.0", "pre": "1.0.0-next.1",
		"caret": "^8.45.0", "wild": "8.x", "hyphen": "1.0.0 - 2.0.0", "or": "1.0.0 || 2.0.0"}}`}
	const pkgFile = "packages/x/package.json"
	tests := []struct {
		name  string
		files map[string][]string
		want  []string // output lines; none means the check passes
	}{
		{"env pin matches", map[string][]string{
			toolEnv: env,
			toolRef: refHeader("v1.2.3", toolPin, toolURL),
		}, nil},
		{"package.json pins, devDependencies and dependencies, tag forms", map[string][]string{
			pkgFile:                        pkg,
			"docs/references/gen-llms.txt": refHeader("8.45.0", pkgFile+" gen", "https://e.com/blob/v8.45.0/a.md", "https://unpkg.com/gen@8.45.0", "https://e.com/8.45.0"),
			"docs/references/lib-llms.txt": refHeader("2.0.0", pkgFile+" lib", "https://e.com/tree/2.0.0/docs#x"),
			"docs/references/pre-llms.txt": refHeader("1.0.0-next.1", pkgFile+" pre", "https://e.com/v1.0.0-next.1/a"),
		}, nil},
		{"pin moved, reference did not", map[string][]string{
			toolEnv: {"TOOL_VERSION=v1.3.0"},
			toolRef: refHeader("v1.2.3", toolPin, toolURL),
		}, []string{toolRef + ":5: Version v1.2.3, but scripts/tool-versions.env pins TOOL_VERSION v1.3.0: take the file again from upstream at v1.3.0 [docs-references-pinned]"}},
		{"no v-prefix normalization", map[string][]string{
			toolEnv: {"TOOL_VERSION=1.2.3"},
			toolRef: refHeader("v1.2.3", toolPin, toolURL),
		}, []string{toolRef + ":5: Version v1.2.3, but scripts/tool-versions.env pins TOOL_VERSION 1.2.3: take the file again from upstream at 1.2.3 [docs-references-pinned]"}},
		{"source at another version", map[string][]string{
			toolEnv: env,
			toolRef: refHeader("v1.2.3", toolPin, toolURL, "https://example.com/tool/blob/main/x.md"),
		}, []string{toolRef + ":8: Source does not name Version v1.2.3 as a path segment or tag: take the file from upstream at that version [docs-references-pinned]"}},
		{"version inside another version is no match", map[string][]string{
			toolEnv:                         env,
			pkgFile:                         pkg,
			"docs/references/bare-llms.txt": refHeader("1.3.0", toolEnv+" BARE_VERSION", "https://e.com/blob/v11.3.0/a.md"),
			"docs/references/gen-llms.txt":  refHeader("8.45.0", pkgFile+" gen", "https://e.com/18.45.0/a.md"),
			"docs/references/rc-llms.txt":   refHeader("v1.2.3", toolPin, "https://e.com/blob/v1.2.3-rc1/a.md"),
		}, []string{
			"docs/references/bare-llms.txt:7: Source does not name Version 1.3.0 as a path segment or tag: take the file from upstream at that version [docs-references-pinned]",
			"docs/references/gen-llms.txt:7: Source does not name Version 8.45.0 as a path segment or tag: take the file from upstream at that version [docs-references-pinned]",
			"docs/references/rc-llms.txt:7: Source does not name Version v1.2.3 as a path segment or tag: take the file from upstream at that version [docs-references-pinned]",
		}},
		{"stale pin and bad source, in line order", map[string][]string{
			toolEnv: {"TOOL_VERSION=v1.3.0"},
			toolRef: refHeader("v1.2.3", toolPin, "https://e.com/blob/main/a.md"),
		}, []string{
			toolRef + ":5: Version v1.2.3, but scripts/tool-versions.env pins TOOL_VERSION v1.3.0: take the file again from upstream at v1.3.0 [docs-references-pinned]",
			toolRef + ":7: Source does not name Version v1.2.3 as a path segment or tag: take the file from upstream at that version [docs-references-pinned]",
		}},
		{"sources checked without a Pin", map[string][]string{
			toolEnv: env,
			toolRef: refHeader("v1.2.3", "", "https://e.com/blob/main/a.md"),
		}, []string{
			toolRef + ":1: no single `Pin:` line in the first 20 lines [docs-references-pinned]",
			toolRef + ":6: Source does not name Version v1.2.3 as a path segment or tag: take the file from upstream at that version [docs-references-pinned]",
		}},
		{"header lines missing", map[string][]string{
			toolEnv: env,
			toolRef: refHeader("", ""),
		}, []string{
			toolRef + ":1: no single `Version:` line in the first 20 lines [docs-references-pinned]",
			toolRef + ":1: no single `Pin:` line in the first 20 lines [docs-references-pinned]",
			toolRef + ":1: no `Source:` line in the first 20 lines [docs-references-pinned]",
		}},
		{"two Version lines", map[string][]string{
			toolEnv: env,
			toolRef: append([]string{"Version: v1.2.3"}, refHeader("v1.2.3", toolPin, toolURL)...),
		}, []string{toolRef + ":1: no single `Version:` line in the first 20 lines [docs-references-pinned]"}},
		{"key not pinned", map[string][]string{
			toolEnv: env,
			toolRef: refHeader("v1.2.3", toolEnv+" NOPE_VERSION", toolURL),
		}, []string{toolRef + ":6: scripts/tool-versions.env pins no NOPE_VERSION [docs-references-pinned]"}},
		{"env reads plain KEY=value only", map[string][]string{
			toolEnv: {"export TOOL_VERSION=v1.2.3"},
			toolRef: refHeader("v1.2.3", toolPin, toolURL),
		}, []string{toolRef + ":6: scripts/tool-versions.env pins no TOOL_VERSION [docs-references-pinned]"}},
		{"pin without key", map[string][]string{
			toolEnv: env,
			toolRef: refHeader("v1.2.3", toolEnv, toolURL),
		}, []string{toolRef + `:6: Pin "scripts/tool-versions.env" is not ` + "`<file> <key>`" + ` [docs-references-pinned]`}},
		{"unsupported pin file", map[string][]string{
			"go.mod": {"module x"},
			toolRef:  refHeader("v1.2.3", "go.mod tool", toolURL),
		}, []string{toolRef + ":6: pin file go.mod is neither a *.env (KEY=value) nor a package.json [docs-references-pinned]"}},
		{"pin file missing", map[string][]string{
			toolRef: refHeader("v1.2.3", toolPin, toolURL),
		}, []string{toolRef + ":6: pin file scripts/tool-versions.env is not a git-known file in the repo [docs-references-pinned]"}},
		{"pin file outside the repo", map[string][]string{
			toolEnv: env,
			toolRef: refHeader("v1.2.3", "../tool-versions.env TOOL_VERSION", toolURL),
		}, []string{toolRef + ":6: pin file ../tool-versions.env is not a git-known file in the repo [docs-references-pinned]"}},
		{"pin path with ..", map[string][]string{
			toolEnv: env,
			toolRef: refHeader("v1.2.3", "docs/../scripts/tool-versions.env TOOL_VERSION", toolURL),
		}, []string{toolRef + ":6: pin file docs/../scripts/tool-versions.env is not a git-known file in the repo [docs-references-pinned]"}},
		{"absolute pin path", map[string][]string{
			toolEnv: env,
			toolRef: refHeader("v1.2.3", "/etc/tool.env TOOL_VERSION", toolURL),
		}, []string{toolRef + ":6: pin file /etc/tool.env is not a git-known file in the repo [docs-references-pinned]"}},
		{"ignored pin file is not read (no secret printed)", map[string][]string{
			".gitignore":    {"/apps/api/.env"},
			"apps/api/.env": {"TOOL_VERSION=s3cret"},
			toolRef:         refHeader("v1.2.3", "apps/api/.env TOOL_VERSION", toolURL),
		}, []string{toolRef + ":6: pin file apps/api/.env is not a git-known file in the repo [docs-references-pinned]"}},
		{"package.json ranges", map[string][]string{
			pkgFile:                           pkg,
			"docs/references/caret-llms.txt":  refHeader("8.45.0", pkgFile+" caret", "https://e.com/v8.45.0/a"),
			"docs/references/hyphen-llms.txt": refHeader("1.0.0", pkgFile+" hyphen", "https://e.com/v1.0.0/a"),
			"docs/references/or-llms.txt":     refHeader("1.0.0", pkgFile+" or", "https://e.com/v1.0.0/a"),
			"docs/references/wild-llms.txt":   refHeader("8.45.0", pkgFile+" wild", "https://e.com/v8.45.0/a"),
		}, []string{
			"docs/references/caret-llms.txt:6: packages/x/package.json pins caret as a range ^8.45.0, not a version: pin it exactly [docs-references-pinned]",
			"docs/references/hyphen-llms.txt:6: packages/x/package.json pins hyphen as a range 1.0.0 - 2.0.0, not a version: pin it exactly [docs-references-pinned]",
			"docs/references/or-llms.txt:6: packages/x/package.json pins or as a range 1.0.0 || 2.0.0, not a version: pin it exactly [docs-references-pinned]",
			"docs/references/wild-llms.txt:6: packages/x/package.json pins wild as a range 8.x, not a version: pin it exactly [docs-references-pinned]",
		}},
		{"files outside docs/references are ignored", map[string][]string{
			toolEnv:                          env,
			toolRef:                          refHeader("v1.2.3", toolPin, toolURL),
			"docs/references/old/x-llms.txt": {"no header"},
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			bad, err := referencesPinned(testWorld(t, &out), linkRepo(t, tt.files))
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
			if out.Len() == 0 {
				got = nil
			}
			if bad != (len(tt.want) > 0) || strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("bad=%v, output:\n%s\nwant:\n%s", bad, out.String(), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestReferencesPinnedCannotRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args func(t *testing.T) []string
		want int
	}{
		{"missing root", func(*testing.T) []string { return []string{"references-pinned"} }, exitUsage},
		{"not a git repository", func(t *testing.T) []string { return []string{"references-pinned", t.TempDir()} }, 2},
		{"no reference files", func(t *testing.T) []string {
			return []string{"references-pinned", linkRepo(t, map[string][]string{"README.md": {"x"}})}
		}, 2},
		{"git-known package.json does not parse", func(t *testing.T) []string {
			return []string{"references-pinned", linkRepo(t, map[string][]string{
				"package.json":                 {"{"},
				"docs/references/gen-llms.txt": refHeader("1.0.0", "package.json gen", "https://e.com/1.0.0"),
			})}
		}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := runTest(t, tt.args(t), strings.NewReader(""), &stdout, &stderr); got != tt.want {
				t.Errorf("exit %d, want %d; stdout %q stderr %q", got, tt.want, stdout.String(), stderr.String())
			}
		})
	}
}
