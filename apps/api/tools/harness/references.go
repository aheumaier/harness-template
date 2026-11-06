package main

// references-pinned: every docs/references/<tool>-llms.txt names, in its header, the tool version it was
// taken from (`Version:`), where the repo pins that tool (`Pin: <file> <key>`) and the upstream URLs it
// came from (`Source:`, each naming that version as a whole path segment or tag). The version must equal
// the pin, so raising a pin without taking the reference again from upstream fails (docs-references-pinned).
// Run from `make references-pinned`, part of `make check`. Only the header is checked; whether the body
// matches upstream is the job of whoever takes the file again.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	referencesDir = "docs/references"
	// headerLines bounds where Version/Pin/Source count, so body text cannot satisfy or break the header.
	headerLines = 20
)

var (
	headerRE = regexp.MustCompile(`^(Version|Pin|Source):\s*(.*?)\s*$`)
	// npmRangeRE matches a package.json value that is a range, not one exact version: an operator, a space
	// (hyphen ranges, `||`), or an x/X wildcard segment.
	npmRangeRE = regexp.MustCompile(`[\^~<>=*\s]|\|\||(^|\.)[xX](\.|$)`)
)

type headerField struct {
	line  int
	value string
}

type finding struct {
	line int
	msg  string
}

func init() {
	commands.register(rootSensor("references-pinned", "docs-references-pinned (make references-pinned)", referencesPinned))
}

// referencesPinned prints one line per finding to stdout and reports whether there was any. root is the repo.
func referencesPinned(w world, root string) (bool, error) {
	known, err := w.gitKnown(root, referencesDir+"/*-llms.txt")
	if err != nil {
		return false, err
	}
	var refs []string
	for _, f := range known {
		if path.Dir(f) == referencesDir {
			refs = append(refs, f)
		}
	}
	if len(refs) == 0 {
		return false, fmt.Errorf("no %s/*-llms.txt known to git: run from the repository root", referencesDir)
	}
	c := &refChecker{w: w, root: root, pins: map[string]map[string]string{}}
	bad := false
	for _, ref := range refs {
		findings, err := c.check(ref)
		if err != nil {
			return false, err
		}
		// One file's findings in line order, so the header reads top to bottom.
		sort.SliceStable(findings, func(i, j int) bool { return findings[i].line < findings[j].line })
		for _, f := range findings {
			bad = true
			printFinding(w.stdout, ref, f.line, "docs-references-pinned", "%s", f.msg)
		}
	}
	return bad, nil
}

type refChecker struct {
	w    world
	root string
	pins map[string]map[string]string // pin file → key → version, read once each
}

func (c *refChecker) check(ref string) ([]finding, error) {
	version, pin, sources, err := readHeader(filepath.Join(c.root, ref))
	if err != nil {
		return nil, err
	}
	var out []finding
	add := func(line int, format string, a ...any) {
		out = append(out, finding{line, fmt.Sprintf(format, a...)})
	}
	if version == nil {
		add(1, "no single `Version:` line in the first %d lines", headerLines)
	}
	if pin == nil {
		add(1, "no single `Pin:` line in the first %d lines", headerLines)
	}
	if len(sources) == 0 {
		add(1, "no `Source:` line in the first %d lines", headerLines)
	}
	if version == nil {
		return out, nil
	}
	if pin != nil {
		f, err := c.comparePin(version, pin)
		if err != nil {
			return nil, err
		}
		out = append(out, f...)
	}
	names := versionSegmentRE(version.value)
	for _, s := range sources {
		if !names.MatchString(s.value) {
			add(s.line, "Source does not name Version %s as a path segment or tag: take the file from upstream at that version", version.value)
		}
	}
	return out, nil
}

// comparePin checks the Pin line and compares the version it points at with Version.
func (c *refChecker) comparePin(version, pin *headerField) ([]finding, error) {
	at := func(line int, format string, a ...any) []finding {
		return []finding{{line, fmt.Sprintf(format, a...)}}
	}
	file, key, ok := strings.Cut(pin.value, " ")
	key = strings.TrimSpace(key)
	if !ok || file == "" || key == "" || strings.ContainsAny(key, " \t") {
		return at(pin.line, "Pin %q is not `<file> <key>`", pin.value), nil
	}
	values, ok := c.pins[file]
	if !ok {
		// Only a git-known file inside the repo is read: an ignored local file (.env.local) could print a
		// secret on a mismatch, and a path outside the repo is not the repo's pin.
		safe, err := c.knownPinFile(file)
		if err != nil {
			return nil, err
		}
		if !safe {
			return at(pin.line, "pin file %s is not a git-known file in the repo", file), nil
		}
		if !strings.HasSuffix(file, ".env") && path.Base(file) != "package.json" {
			return at(pin.line, "pin file %s is neither a *.env (KEY=value) nor a package.json", file), nil
		}
		if values, err = readPins(c.root, file); err != nil {
			return nil, err
		}
		c.pins[file] = values
	}
	want, found := values[key]
	switch {
	case !found:
		return at(pin.line, "%s pins no %s", file, key), nil
	case path.Base(file) == "package.json" && npmRangeRE.MatchString(want):
		return at(pin.line, "%s pins %s as a range %s, not a version: pin it exactly", file, key, want), nil
	case want != version.value:
		return at(version.line, "Version %s, but %s pins %s %s: take the file again from upstream at %s", version.value, file, key, want, want), nil
	}
	return nil, nil
}

// knownPinFile reports whether file is a clean relative path (no `..`) that git knows (tracked, or untracked
// and not ignored, as every harness sensor reads the repo).
func (c *refChecker) knownPinFile(file string) (bool, error) {
	if filepath.IsAbs(file) || path.Clean(file) != file || file == ".." || strings.HasPrefix(file, "../") {
		return false, nil
	}
	known, err := c.w.gitKnown(c.root, ":(literal)"+file)
	if err != nil {
		return false, err
	}
	return len(known) == 1 && known[0] == file, nil
}

// versionSegmentRE matches v as a whole path segment or tag in a URL (`/v1.3.0/`, `@8.45.0`, with or without a
// `v` prefix), not as part of another version (`v11.3.0`, `/18.45.0/`, `v1.3.0-rc1`).
func versionSegmentRE(v string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[/@])v?` + regexp.QuoteMeta(v) + `($|[/#?])`)
}

// readHeader returns the Version and Pin lines (nil unless exactly one each) and every Source line among the
// first headerLines lines of the file.
func readHeader(file string) (version, pin *headerField, sources []headerField, err error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()
	var versions, pinLines []headerField
	sc := bufio.NewScanner(f)
	for n := 1; n <= headerLines && sc.Scan(); n++ {
		m := headerRE.FindStringSubmatch(sc.Text())
		if m == nil || m[2] == "" {
			continue
		}
		field := headerField{n, m[2]}
		switch m[1] {
		case "Version":
			versions = append(versions, field)
		case "Pin":
			pinLines = append(pinLines, field)
		default:
			sources = append(sources, field)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", file, err)
	}
	if len(versions) == 1 {
		version = &versions[0]
	}
	if len(pinLines) == 1 {
		pin = &pinLines[0]
	}
	return version, pin, sources, nil
}

// readPins reads the versions a git-known pin file sets: plain KEY=value lines of a *.env file (the subset
// scripts/tool-versions.env allows: no spaces, no quotes; blank and # lines skipped), or the dependencies and
// devDependencies of a package.json. A pin file that cannot be read or parsed is an error: the check cannot
// run (fail closed).
func readPins(root, file string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return nil, fmt.Errorf("read pin file: %w", err)
	}
	values := map[string]string{}
	if strings.HasSuffix(file, ".env") {
		for _, line := range bytes.Split(b, []byte("\n")) {
			s := strings.TrimSpace(string(line))
			if s == "" || strings.HasPrefix(s, "#") {
				continue
			}
			if k, v, ok := strings.Cut(s, "="); ok {
				values[k] = v
			}
		}
		return values, nil
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	for k, v := range pkg.DevDependencies {
		values[k] = v
	}
	for k, v := range pkg.Dependencies {
		values[k] = v
	}
	return values, nil
}
