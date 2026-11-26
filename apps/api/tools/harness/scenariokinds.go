package main

// Which tests run the app, for specs-tested (specs-scenarios-tested, ADR 0049). With db in STACK, a scenario
// without `(unit: …)` or `(untested: …)` is one a user sees, and only a test of a kind STACK runs traces it:
//   - a journey: a *.e2e.ts file under apps/web/e2e/ at any depth, as Playwright's testDir and testMatch collect
//     it (make e2e, db and web in STACK). The web app's directory has no layout.env key yet (TD-46);
//   - an integration test: a *_test.go directly in <domains dir>/<domain>/api/ (the domain's api package; a
//     deeper directory is another package) that `go test -race -tags=integration` builds and `go test -race`
//     (make test-go) does not, as go/build decides it for GOOS and GOARCH and the race tag (make
//     test-integration, db in STACK). A domain directory that `./...` skips (vendor, testdata, a name starting with `_` or `.`, or one
//     inside another module: a go.mod below API_DIR) runs nothing. cgo and the other tool tags are the host's: an
//     integration test that needs cgo is counted although a GOOS=linux cross build would skip it, and a go.mod
//     `ignore` line is not read (TD-180).
//
// Without db no test runs the app, so any test traces such a scenario, and the sensor says so in a notice.
// TestAppTestKindsMatchMake keeps these kinds in step with the Makefile and playwright.config.ts.

import (
	"errors"
	"fmt"
	"go/build"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const (
	webAppDir      = "apps/web" // where the Makefile's e2e target runs Playwright
	journeyDir     = webAppDir + "/e2e/"
	journeySuffix  = ".e2e.ts"
	integrationTag = "integration"
)

// appTests decides which test files run the app under one STACK.
type appTests struct {
	journeys    bool   // db and web in STACK
	integration bool   // db in STACK
	apiDir      string // set with integration
	domainsDir  string // set with integration
	stackAt     string // where STACK came from, for the notice
	goos        string
	goarch      string
}

// loadAppTests reads STACK and the domains directory from root's layout.env, and GOOS and GOARCH from the
// environment (the host's when unset), as `go test` does.
func loadAppTests(w world, root string) (appTests, error) {
	l, err := loadLayout(w, root)
	if err != nil {
		return appTests{}, err
	}
	at := appTests{journeys: l.has("db") && l.has("web"), integration: l.has("db"), stackAt: l.stackAt, goos: w.getenv("GOOS"), goarch: w.getenv("GOARCH")}
	if at.goos == "" {
		at.goos = runtime.GOOS
	}
	if at.goarch == "" {
		at.goarch = runtime.GOARCH
	}
	if at.integration {
		at.apiDir = l.apiPath(".")
		at.domainsDir, err = l.domainsDir()
	}
	return at, err
}

// kinds says in words which tests trace a scenario a user sees, for the finding.
func (at appTests) kinds() string {
	integration := "an integration test in " + at.domainsDir + "/<domain>/api/ (//go:build " + integrationTag + ")"
	if at.journeys {
		return "a journey (" + journeyDir + "**/*" + journeySuffix + ") or " + integration
	}
	return integration
}

// notice is the line printed when STACK has no test that runs the app.
func (at appTests) notice() string {
	return fmt.Sprintf("notice: %s: no db, so no test runs the app and any test traces a scenario a user sees (ADR 0049) [%s]", at.stackAt, scenarioRule)
}

// runsApp reports whether the product test file rel, a regular file under root, runs the app.
func (at appTests) runsApp(root, rel string) (bool, error) {
	if at.journeys && strings.HasPrefix(rel, journeyDir) && strings.HasSuffix(rel, journeySuffix) {
		return true, nil
	}
	pkg := path.Dir(rel)
	domain := path.Dir(pkg)
	if !at.integration || !strings.HasSuffix(rel, "_test.go") || path.Base(pkg) != "api" ||
		path.Dir(domain) != at.domainsDir || !isDomainDir(path.Base(domain)) || !goListed(path.Base(domain)) {
		return false, nil
	}
	for d := pkg; d != at.apiDir && d != "."; d = path.Dir(d) {
		_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(d), "go.mod"))
		if err == nil {
			return false, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH = at.goos, at.goarch
	ctx.ToolTags = append(slices.Clip(ctx.ToolTags), "race") // make test-integration runs go test -race
	dir, name := filepath.Join(root, filepath.FromSlash(pkg)), path.Base(rel)
	without, err := ctx.MatchFile(dir, name)
	if err != nil || without {
		return false, err
	}
	ctx.BuildTags = []string{integrationTag}
	return ctx.MatchFile(dir, name)
}

// goListed reports whether `go test ./...` walks into the directory name (go help packages).
func goListed(name string) bool {
	return !strings.HasPrefix(name, "_") && !strings.HasPrefix(name, ".") && name != "testdata" && name != "vendor"
}
