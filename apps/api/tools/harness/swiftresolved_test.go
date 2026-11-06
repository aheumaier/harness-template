package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// lockfile is a Package.resolved of version 3 with one pin.
const lockfile = `{"pins": [{"identity": "b", "kind": "remoteSourceControl", "location": "https://github.com/a/b"}], "version": 3}`

func TestSwiftResolved(t *testing.T) {
	t.Parallel()
	const remote = `.package(url: "https://github.com/a/b", exact: "1.0.0")`
	manifest := func(deps ...string) []string {
		lines := []string{"// swift-tools-version: 6.0", "import PackageDescription", "let package = Package(", "  name: \"P\",", "  dependencies: ["}
		lines = append(lines, deps...)
		return append(lines, "  ]", ")")
	}
	missing := func(m, resolved string) string {
		return m + ":1: declares a remote package, but " + resolved + " is not a file in git that pins one [deps-swift-resolved]"
	}
	tests := []struct {
		name   string
		files  map[string][]string
		staged []string // files added to the index; the rest stay untracked
		want   []string
	}{
		{"local packages only", map[string][]string{
			"apps/ios/Package.swift": manifest(`.package(path: "../Shared"),`, `.package( path : "x")`),
		}, nil, nil},
		{"no dependencies", map[string][]string{"Package.swift": manifest()}, nil, nil},
		{"remote with a tracked Package.resolved", map[string][]string{
			"apps/ios/Package.swift":    manifest(remote),
			"apps/ios/Package.resolved": {lockfile},
		}, []string{"apps/ios/Package.resolved"}, nil},
		{"remote with a tracked Package.resolved of version 1", map[string][]string{
			"Package.swift":    manifest(remote),
			"Package.resolved": {`{"object": {"pins": [{"package": "b"}]}, "version": 1}`},
		}, []string{"Package.resolved"}, nil},
		{"a Package.resolved that pins nothing does not count", map[string][]string{
			"Package.swift":    manifest(remote),
			"Package.resolved": {`{"pins": [], "version": 3}`},
		}, []string{"Package.resolved"}, []string{missing("Package.swift", "Package.resolved")}},
		{"remote without Package.resolved", map[string][]string{
			"apps/ios/Package.swift": manifest(remote),
		}, nil, []string{missing("apps/ios/Package.swift", "apps/ios/Package.resolved")}},
		{"untracked Package.resolved does not count (swift test writes it)", map[string][]string{
			"apps/ios/Package.swift":    manifest(remote),
			"apps/ios/Package.resolved": {lockfile},
		}, nil, []string{missing("apps/ios/Package.swift", "apps/ios/Package.resolved")}},
		{"Package.resolved in another folder does not count", map[string][]string{
			"apps/ios/Package.swift": manifest(remote),
			"Package.resolved":       {lockfile},
		}, []string{"Package.resolved"}, []string{missing("apps/ios/Package.swift", "apps/ios/Package.resolved")}},
		{"registry id, line breaks and the old name: form are remote", map[string][]string{
			"a/Package.swift": manifest(`.package(id: "a.b", from: "1.0.0")`),
			"b/Package.swift": manifest(".package(", `  url: "https://x/y",`, `  from: "1.0.0")`),
			"c/Package.swift": manifest(`.package(name: "Y", url: "https://x/y", from: "1.0.0")`),
		}, nil, []string{
			missing("a/Package.swift", "a/Package.resolved"),
			missing("b/Package.swift", "b/Package.resolved"),
			missing("c/Package.swift", "c/Package.resolved"),
		}},
		{"a local package does not hide a remote one", map[string][]string{
			"Package.swift": manifest(`.package(path: "x"),`, remote),
		}, nil, []string{missing("Package.swift", "Package.resolved")}},
		{"backticks and a comment before the parenthesis are remote", map[string][]string{
			"a/Package.swift": manifest(".`package`(url: \"https://x/y\", from: \"1.0.0\")"),
			"b/Package.swift": manifest(`.package/* x */(url: "https://x/y", from: "1.0.0")`),
		}, nil, []string{missing("a/Package.swift", "a/Package.resolved"), missing("b/Package.swift", "b/Package.resolved")}},
		{"package( without the dot, in an extension of Package.Dependency, is remote", map[string][]string{
			"Package.swift": manifest(`.collections`, `]`, `extension Package.Dependency {`,
				`  static let collections = package(url: "https://x/y", from: "1.0.0")`, `}`, `let x = [`),
		}, nil, []string{missing("Package.swift", "Package.resolved")}},
		{"a longer name is no package", map[string][]string{
			"Package.swift": manifest(`// a local Swift package (ADR 0041)`, `.package(path: "x"),`, `.packageAccess(true),`, `.product(name: "A", package: "x")`),
		}, nil, nil},
		{"a manifest in another case is a manifest (macOS)", map[string][]string{
			"apps/ios/package.swift": manifest(remote),
		}, nil, []string{missing("apps/ios/package.swift", "apps/ios/Package.resolved")}},
		{"an empty Package.resolved (white space only) does not count", map[string][]string{
			"Package.swift":    manifest(remote),
			"Package.resolved": {},
		}, []string{"Package.resolved"}, []string{missing("Package.swift", "Package.resolved")}},
		{"a commented-out remote package counts (fails closed)", map[string][]string{
			"Package.swift": manifest("// " + remote),
		}, nil, []string{missing("Package.swift", "Package.resolved")}},
		{"version-specific manifest", map[string][]string{
			"apps/ios/Package.swift":           manifest(),
			"apps/ios/Package@swift-5.9.swift": manifest(remote),
		}, nil, []string{missing("apps/ios/Package@swift-5.9.swift", "apps/ios/Package.resolved")}},
		{"fixtures and ignored folders are skipped", map[string][]string{
			".gitignore": {".build/"},
			"scripts/harness/fixtures/x/Package.swift":  manifest(remote),
			"apps/ios/.build/checkouts/b/Package.swift": manifest(remote),
		}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := linkRepo(t, tt.files)
			for _, f := range tt.staged {
				gitT(t, root, "add", "--", f)
			}
			wantFindings(t, func(out io.Writer) (bool, error) { return swiftResolved(testWorld(t, out), root) }, tt.want)
		})
	}
}

func TestSwiftResolvedSymlink(t *testing.T) {
	t.Parallel()
	root := linkRepo(t, map[string][]string{
		"Package.swift": {`.package(url: "https://github.com/a/b", exact: "1.0.0")`},
	})
	if err := os.Symlink("/dev/null", filepath.Join(root, "Package.resolved")); err != nil {
		t.Fatal(err)
	}
	gitT(t, root, "add", "--", "Package.resolved")
	wantFindings(t, func(out io.Writer) (bool, error) { return swiftResolved(testWorld(t, out), root) },
		[]string{"Package.swift:1: declares a remote package, but Package.resolved is not a file in git that pins one [deps-swift-resolved]"})
}

func TestSwiftResolvedIntentToAdd(t *testing.T) {
	t.Parallel()
	root := linkRepo(t, map[string][]string{
		"Package.swift":    {`.package(url: "https://github.com/a/b", exact: "1.0.0")`},
		"Package.resolved": {lockfile},
	})
	gitT(t, root, "add", "-N", "--", "Package.resolved") // stages the empty blob
	wantFindings(t, func(out io.Writer) (bool, error) { return swiftResolved(testWorld(t, out), root) },
		[]string{"Package.swift:1: declares a remote package, but Package.resolved is not a file in git that pins one [deps-swift-resolved]"})
}

func TestSwiftResolvedLinkedManifest(t *testing.T) {
	t.Parallel()
	root := linkRepo(t, map[string][]string{"x.swift": {`.package(url: "https://github.com/a/b", exact: "1.0.0")`}})
	if err := os.Symlink("x.swift", filepath.Join(root, "Package.swift")); err != nil {
		t.Fatal(err)
	}
	wantFindings(t, func(out io.Writer) (bool, error) { return swiftResolved(testWorld(t, out), root) },
		[]string{"Package.swift:1: is not a regular file, so it is not read: make it one [deps-swift-resolved]"})
}

func TestSwiftResolvedStagedLink(t *testing.T) {
	t.Parallel()
	root := linkRepo(t, map[string][]string{
		"Package.swift": {`.package(url: "https://github.com/a/b", exact: "1.0.0")`},
	})
	resolved := filepath.Join(root, "Package.resolved")
	if err := os.Symlink("/dev/null", resolved); err != nil {
		t.Fatal(err)
	}
	gitT(t, root, "add", "--", "Package.resolved")
	// The working tree holds a lockfile, but git's index holds the link.
	if err := os.Remove(resolved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolved, []byte(lockfile), 0o644); err != nil {
		t.Fatal(err)
	}
	wantFindings(t, func(out io.Writer) (bool, error) { return swiftResolved(testWorld(t, out), root) },
		[]string{"Package.swift:1: declares a remote package, but Package.resolved is not a file in git that pins one [deps-swift-resolved]"})
}
