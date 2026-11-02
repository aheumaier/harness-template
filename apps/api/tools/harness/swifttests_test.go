package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwiftTests(t *testing.T) {
	const describe = `{"targets": [
		{"name": "AppCoreTests", "path": "Tests/AppCoreTests", "sources": ["GreetingTests.swift", "Sub/MoreTests.swift"], "type": "test"},
		{"name": "Support", "path": "Tests/Support", "sources": ["Helper.swift"], "type": "library"},
		{"name": "AppCore", "path": "Sources/AppCore", "sources": ["Greeting.swift"], "type": "library"}]}`
	notRun := func(f string) string {
		return f + ":1: no test target of apps/ios/Package.swift compiles it, so swift test never runs it [ios-tests-pass]"
	}
	tests := []struct {
		name     string
		files    []string
		describe string
		want     []string
	}{
		{"every test is in a test target", []string{
			"apps/ios/Tests/AppCoreTests/GreetingTests.swift",
			"apps/ios/Tests/AppCoreTests/Sub/MoreTests.swift",
			"apps/ios/Sources/AppCore/Greeting.swift",
			"apps/ios/Sources/AppCore/Loose.swift", // not a test: other sensors' business
		}, describe, nil},
		{"undeclared folder, excluded file, library target", []string{
			"apps/ios/Tests/AppCoreTests/GreetingTests.swift",
			"apps/ios/Tests/LooseTests/LooseTests.swift",
			"apps/ios/Tests/AppCoreTests/Excluded.swift",
			"apps/ios/Tests/Support/Helper.swift",
		}, describe, []string{
			notRun("apps/ios/Tests/AppCoreTests/Excluded.swift"),
			notRun("apps/ios/Tests/LooseTests/LooseTests.swift"),
			notRun("apps/ios/Tests/Support/Helper.swift"),
		}},
		{"no targets", []string{"apps/ios/Tests/X/XTests.swift"}, `{"targets": []}`,
			[]string{notRun("apps/ios/Tests/X/XTests.swift")}},
		{"tests of another folder are not this package's", []string{
			"apps/ios/Shared/Tests/SharedTests/SharedTests.swift",
			"Tests/RootTests/RootTests.swift",
		}, describe, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string][]string{}
			for _, f := range tt.files {
				files[f] = []string{"import Testing"}
			}
			root := linkRepo(t, files)
			wantFindings(t, func(out io.Writer) (bool, error) {
				return swiftTests(root, "apps/ios", strings.NewReader(tt.describe), out)
			}, tt.want)
		})
	}
}

// SwiftPM follows a link under or at Tests/, but git lists only the link: the files behind it are unchecked.
func TestSwiftTestsLinks(t *testing.T) {
	for _, tt := range []struct{ name, link, target string }{
		{"Tests itself", "apps/ios/Tests", "TestSuite"},
		{"a folder under Tests", "apps/ios/Tests/OrphanTests", "../TestSuite/OrphanTests"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := linkRepo(t, map[string][]string{"apps/ios/TestSuite/OrphanTests/XTests.swift": {"import Testing"}})
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, tt.link)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(tt.target, filepath.Join(root, tt.link)); err != nil {
				t.Fatal(err)
			}
			wantFindings(t, func(out io.Writer) (bool, error) {
				return swiftTests(root, "apps/ios", strings.NewReader(`{"targets": []}`), out)
			}, []string{tt.link + ":1: is a link, so the tests behind it are not checked: make it a folder or a file [ios-tests-pass]"})
		})
	}
}

// make test-ios reads exit 1 as a finding and 2 as a broken sensor; a broken describe is never "no finding".
func TestSwiftTestsExitCodes(t *testing.T) {
	root := linkRepo(t, map[string][]string{"apps/ios/Tests/AppCoreTests/GreetingTests.swift": {"import Testing"}})
	const declared = `{"targets": [{"path": "Tests/AppCoreTests", "sources": ["GreetingTests.swift"], "type": "test"}]}`
	for _, tt := range []struct {
		args  []string
		stdin string
		want  int
	}{
		{[]string{"swift-tests", root, "apps/ios"}, declared, 0},
		{[]string{"swift-tests", root, "apps/ios"}, `{"targets": []}`, 1},
		{[]string{"swift-tests", root, "apps/ios"}, "", 2},
		{[]string{"swift-tests", root, "apps/ios"}, "error: manifest parse error", 2},
		{[]string{"swift-tests", root, "apps/ios"}, `{"targets": 1}`, 2},
		{[]string{"swift-tests", root}, declared, 2},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr); code != tt.want {
			t.Errorf("%v <<< %q: exit %d, want %d (%s%s)", tt.args[1:], tt.stdin, code, tt.want, stdout.String(), stderr.String())
		}
	}
}
