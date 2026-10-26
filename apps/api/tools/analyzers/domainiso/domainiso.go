// Package domainiso enforces domain isolation inside apps/api/internal.
//
// Rule (docs/harness/rules.md#go-domain-isolation): a package in internal/<a>/... must not import
// internal/<b>/store or internal/<b>/api for any other domain b. Cross-domain collaboration goes
// through the other domain's service (or model). go-arch-lint globs cannot express "same domain",
// hence this analyzer.
package domainiso

import (
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const message = "domain %q imports %s of domain %q [go-domain-isolation]. " +
	"Fix: depend on %s/service (or %s/model) instead; stores and HTTP handlers are private to their domain. " +
	"See docs/harness/rules.md#go-domain-isolation"

// Analyzer reports cross-domain imports of store or api packages.
var Analyzer = &analysis.Analyzer{
	Name: "domainiso",
	Doc:  "forbid importing another domain's store or api package",
	Run:  run,
}

// split returns (domain, layer) for paths like ".../internal/<domain>/<layer>/...".
func split(path string) (domain, layer string, ok bool) {
	i := strings.LastIndex(path, "/internal/")
	if i < 0 {
		return "", "", false
	}
	parts := strings.Split(path[i+len("/internal/"):], "/")
	if len(parts) < 2 || parts[0] == "platform" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func run(pass *analysis.Pass) (any, error) {
	from, _, ok := split(pass.Pkg.Path())
	if !ok {
		return nil, nil
	}
	for _, f := range pass.Files {
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			to, layer, ok := split(path)
			if ok && to != from && (layer == "store" || layer == "api") {
				pass.Reportf(imp.Pos(), message, from, layer, to, to, to)
			}
		}
	}
	return nil, nil
}
