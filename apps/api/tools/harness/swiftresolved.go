package main

// swift-resolved (deps-swift-resolved, make swift-resolved in make check, ADR 0044): a git-known Swift package
// manifest (Package.swift, or a version-specific Package@swift-<version>.swift, in any case: macOS's file system
// reads package.swift as Package.swift) that declares a remote package needs a non-empty Package.resolved beside
// it in git's index. Without one, SwiftPM resolves the newest versions the requirements allow on every fresh
// checkout, so CI builds code nobody reviewed, and Trivy (deps-no-known-vulns) has no lockfile to scan.
// Untracked does not count: `swift test` writes the file itself.
//
// The manifest is Swift, so it is read as text, not run: with white space and backticks removed, every
// `.package` that is not `.package(path:` declares a remote package (url:, id:, the old name: first form, a
// comment before the parenthesis), and so does `package(url:`, `package(id:` or `package(name:` without the dot,
// as in an extension of Package.Dependency. A commented-out dependency counts too: failing closed beats parsing
// Swift. Prose such as "a Swift package (ADR 0041)" does not, so the dotless form is missed when a comment
// separates `package` from `(url:`. The lockfile must pin at least one package, as every lockfile of a remote
// dependency does. Fixtures are skipped (skippedPath).

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

func init() {
	commands.register(rootSensor("swift-resolved", "deps-swift-resolved: Package.resolved in git (make swift-resolved)", swiftResolved))
}

func swiftResolved(w world, root string) (bool, error) {
	manifests, err := w.gitFiles(root, ":(icase)Package.swift", ":(icase)*/Package.swift",
		":(icase)Package@swift-*.swift", ":(icase)*/Package@swift-*.swift")
	if err != nil {
		return false, err
	}
	bad := false
	for _, m := range manifests {
		// A link can point anywhere, a FIFO that never ends a read included; SwiftPM follows it, so it fails.
		if regular, err := isRegular(root, m); err != nil || !regular {
			printFinding(w.stdout, m, 1, "deps-swift-resolved", "is not a regular file, so it is not read: make it one")
			bad = true
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, m))
		if err != nil {
			return false, err
		}
		if !declaresRemotePackage(src) {
			continue
		}
		resolved := path.Join(path.Dir(m), "Package.resolved")
		if ok, err := lockfileInGit(w, root, resolved); err != nil {
			return false, err
		} else if !ok {
			printFinding(w.stdout, m, 1, "deps-swift-resolved", "declares a remote package, but %s is not a file in git that pins one", resolved)
			bad = true
		}
	}
	return bad, nil
}

// lockfileInGit reports whether f is a regular file in git's index and in the working tree that pins at least
// one package. A symlink is no lockfile (it can point anywhere, /dev/null included), and `git add -N` stages the
// empty blob.
func lockfileInGit(w world, root, f string) (bool, error) {
	if regular, err := isRegular(root, f); err != nil || !regular {
		return false, nil
	}
	src, err := os.ReadFile(filepath.Join(root, f))
	if err != nil {
		return false, err
	}
	var lock struct {
		Pins   []json.RawMessage `json:"pins"`
		Object struct {
			Pins []json.RawMessage `json:"pins"`
		} `json:"object"` // version 1
	}
	if json.Unmarshal(src, &lock) != nil || len(lock.Pins)+len(lock.Object.Pins) == 0 {
		return false, nil
	}
	staged, err := w.git(root, "ls-files", "-s", "-z", "--", ":(literal)"+f)
	if err != nil {
		return false, err
	}
	fields := strings.Fields(string(staged)) // mode, blob, stage, path
	if len(fields) < 2 || (fields[0] != "100644" && fields[0] != "100755") {
		return false, nil
	}
	size, err := w.git(root, "cat-file", "-s", fields[1])
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(size)) != "0", nil
}

// declaresRemotePackage reports whether a manifest names `.package` anywhere but in `.package(path:`, or calls
// `package(` with a remote label.
func declaresRemotePackage(src []byte) bool {
	flat := bytes.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '`' { // .`package`(url: is .package(url:
			return -1
		}
		return r
	}, src)
	for i := 0; ; {
		j := bytes.Index(flat[i:], []byte("package"))
		if j < 0 {
			return false
		}
		dotted := i+j > 0 && flat[i+j-1] == '.'
		i += j + len("package")
		rest := flat[i:]
		longer := len(rest) > 0 && (rest[0] == '_' || rest[0] >= 0x80 || unicode.IsLetter(rune(rest[0])) || unicode.IsDigit(rune(rest[0])))
		if dotted && !longer && !bytes.HasPrefix(rest, []byte("(path:")) {
			return true // not a longer name, such as .packageAccess
		}
		// Without the dot only a call with a remote label: prose in a comment ("a Swift package (…") is no call.
		for _, label := range []string{"(url:", "(id:", "(name:"} {
			if bytes.HasPrefix(rest, []byte(label)) {
				return true
			}
		}
	}
}
