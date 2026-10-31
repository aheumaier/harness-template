// Package domainiso enforces domain isolation inside the API module.
//
// Rule (docs/harness/rules.md#go-domain-isolation): a package in <layer dir>/<a>/... must not import
// <layer dir>/<b>/store or <layer dir>/<b>/api for any other domain b. Cross-domain collaboration goes
// through the other domain's service (or model). go-arch-lint globs cannot express "same domain",
// hence this analyzer. <layer dir> is LAYER_GLOB's literal part in scripts/harness/layout.env
// (`internal` for `internal/*/*`), read from the first layout.env above the working directory
// (golangci-lint runs in API_DIR); without one the analyzer fails instead of checking nothing.
package domainiso

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
)

const message = "domain %q imports %s of domain %q [go-domain-isolation]. " +
	"Fix: depend on %s/service (or %s/model) instead; stores and HTTP handlers are private to their domain. " +
	"See docs/harness/rules.md#go-domain-isolation"

const layoutFile = "scripts/harness/layout.env"

// Analyzer reports cross-domain imports of store or api packages.
var Analyzer = &analysis.Analyzer{
	Name: "domainiso",
	Doc:  "forbid importing another domain's store or api package",
	Run:  run,
}

var (
	prefixOnce sync.Once
	prefix     string
	prefixErr  error
)

// layerPrefix is "/<LAYER_GLOB's literal directories>/", e.g. "/internal/", read once per process.
func layerPrefix() (string, error) {
	prefixOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			prefixErr = err
			return
		}
		prefix, prefixErr = findPrefix(wd)
	})
	return prefix, prefixErr
}

// findPrefix reads LAYER_GLOB from the first layout.env in dir or above it.
func findPrefix(dir string) (string, error) {
	for d := dir; ; {
		b, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(layoutFile)))
		if err == nil {
			return parsePrefix(b)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("no %s in %s or above [go-domain-isolation]: run golangci-lint inside the repo", layoutFile, dir)
		}
		d = parent
	}
}

// parsePrefix takes LAYER_GLOB (`<dir>[/<dir>…]/*/*`) from layout.env. It reads only that key: this module
// (golangci-lint's plugin) cannot import tools/harness (a separate module and a main package), whose layout.go
// checks the whole file in make check (ADR 0030). It accepts and rejects the same LAYER_GLOB values as layout.go's layerSegments; both tests run
// tools/harness/testdata/layer-globs.txt.
func parsePrefix(b []byte) (string, error) {
	for _, line := range strings.Split(string(b), "\n") {
		glob, ok := strings.CutPrefix(strings.TrimSpace(line), "LAYER_GLOB=")
		if !ok {
			continue
		}
		segs := strings.Split(glob, "/")
		bad := len(segs) < 3 || segs[len(segs)-1] != "*" || segs[len(segs)-2] != "*"
		if !bad {
			for _, s := range segs[:len(segs)-2] {
				bad = bad || s == "" || s == "." || s == ".." || strings.ContainsAny(s, "*?[")
			}
		}
		if bad {
			return "", fmt.Errorf("%s: LAYER_GLOB=%s is not <dir>[/<dir>…]/*/* [go-domain-isolation]: run make layout-check", layoutFile, glob)
		}
		return "/" + strings.Join(segs[:len(segs)-2], "/") + "/", nil
	}
	return "", fmt.Errorf("%s: no LAYER_GLOB [go-domain-isolation]: run make layout-check", layoutFile)
}

// split returns (domain, layer) for paths like ".../<prefix><domain>/<layer>/...".
func split(path, prefix string) (domain, layer string, ok bool) {
	i := strings.LastIndex(path, prefix)
	if i < 0 {
		return "", "", false
	}
	parts := strings.Split(path[i+len(prefix):], "/")
	if len(parts) < 2 || parts[0] == "platform" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func run(pass *analysis.Pass) (any, error) {
	prefix, err := layerPrefix()
	if err != nil {
		return nil, err
	}
	from, _, ok := split(pass.Pkg.Path(), prefix)
	if !ok {
		return nil, nil
	}
	for _, f := range pass.Files {
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			to, layer, ok := split(path, prefix)
			if ok && to != from && (layer == "store" || layer == "api") {
				pass.Reportf(imp.Pos(), message, from, layer, to, to, to)
			}
		}
	}
	return nil, nil
}
