package main

// swift-tests (ios-tests-pass, run by make test-ios before swift test, ADR 0044): every git-known *.swift under
// <dir>/Tests/ is a source of a test target of the package in <dir>, as `swift package describe --type json`
// (read from stdin) lists them. swift test compiles only what the manifest SwiftPM picks declares, so a test
// in an undeclared folder, a file the manifest `exclude:`s, or one only a Package@swift-<version>.swift declares
// would pass unrun, and still satisfy specs-scenarios-tested. A file compiled into a library or executable target
// does not count: its tests never run. Tests of a nested package and an XcodeGen app's <App>Tests/ are outside
// <dir>/Tests/ and not checked (TD-153).

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// swiftPackage is the part of `swift package describe --type json` this check reads. A target's path is
// relative to the package, its sources relative to the target's path.
type swiftPackage struct {
	Targets []struct {
		Path    string   `json:"path"`
		Sources []string `json:"sources"`
		Type    string   `json:"type"`
	} `json:"targets"`
}

func init() {
	commands.register(sensor("swift-tests", "harness swift-tests <root> <dir>",
		"ios-tests-pass: every Swift test file is in a declared test target, read from `swift package describe --type json` on stdin (make test-ios)",
		func(w world, args []string) (bool, error) {
			if len(args) != 2 {
				return false, errUsage
			}
			root, err := w.abs(args[0])
			if err != nil {
				return false, err
			}
			return swiftTests(w, root, args[1])
		}))
}

// swiftTests reads the describe output from the world's stdin.
func swiftTests(w world, root, dir string) (bool, error) {
	var pkg swiftPackage
	if err := json.NewDecoder(w.stdin).Decode(&pkg); err != nil {
		return false, fmt.Errorf("reading swift package describe --type json: %w", err)
	}
	compiled := map[string]bool{}
	for _, t := range pkg.Targets {
		if t.Type != "test" {
			continue
		}
		for _, s := range t.Sources {
			compiled[path.Join(dir, t.Path, s)] = true
		}
	}
	// Every git-known path under Tests/, or Tests/ itself when it is a link: SwiftPM follows a link, but git
	// lists only the link, so the files behind it would go unchecked.
	tests, err := w.gitKnown(root, path.Join(dir, "Tests"))
	if err != nil {
		return false, err
	}
	bad := false
	for _, f := range tests {
		if regular, err := isRegular(root, f); err != nil || !regular {
			printFinding(w.stdout, f, 1, "ios-tests-pass", "is a link, so the tests behind it are not checked: make it a folder or a file")
			bad = true
		} else if strings.EqualFold(path.Ext(f), ".swift") && !compiled[f] {
			printFinding(w.stdout, f, 1, "ios-tests-pass", "no test target of %s/Package.swift compiles it, so swift test never runs it", dir)
			bad = true
		}
	}
	return bad, nil
}
