package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// layoutPaths are the paths scripts/harness/layout.env gives (ADR 0030), as `/`-separated segments: API_DIR,
// SPEC, MIGRATIONS_DIR, QUERIES_DIR, API_CLIENT_DIR, the last ones also relative to API_DIR, and LAYER_GLOB's
// literal prefix. A sensor that spells one checks the template's tree, not the product's. `apps/web` has no
// key yet (TD-46).
func layoutPaths(t *testing.T) [][]string {
	t.Helper()
	l, err := loadLayout(testWorld(t, io.Discard), repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	api := l.get("API_DIR")
	values := []string{api, l.get("SPEC")}
	for _, k := range []string{"MIGRATIONS_DIR", "QUERIES_DIR", "API_CLIENT_DIR"} {
		values = append(values, l.get(k))
		if rel, ok := strings.CutPrefix(l.get(k), api+"/"); ok {
			values = append(values, rel)
		}
	}
	prefix, _, _ := strings.Cut(l.get("LAYER_GLOB"), "/*")
	values = append(values, prefix)
	var paths [][]string
	for _, v := range values {
		if v != "" && v != "." {
			paths = append(paths, strings.Split(v, "/"))
		}
	}
	return paths
}

// namesPath reports whether s holds path p segment-aligned: in a word of s (split at anything a path does not
// hold), p's segments appear in a row. `apps/api` is in "under apps/api/internal." and "apps/api", not in
// "apps/apix" or "the api".
func namesPath(s string, p []string) bool {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("/_.-*@+", r)
	})
	for _, w := range words {
		segs := strings.Split(strings.TrimRight(w, "."), "/")
		for i := 0; i+len(p) <= len(segs); i++ {
			if slices.Equal(segs[i:i+len(p)], p) {
				return true
			}
		}
	}
	return false
}

// constString folds a string literal, or a parenthesized one, to its value.
func constString(e ast.Expr) (string, bool) {
	lit, ok := ast.Unparen(e).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// constRuns joins each run of consecutive constant operands with sep: the paths a `+` chain (sep "") or a
// filepath.Join or path.Join call (sep "/") builds from literals, whatever non-constant operands surround them.
func constRuns(operands []ast.Expr, sep string) []string {
	var runs, run []string
	flush := func() {
		if len(run) > 1 {
			runs = append(runs, strings.Join(run, sep))
		}
		run = nil
	}
	for _, e := range operands {
		if s, ok := constString(e); ok {
			run = append(run, s)
		} else {
			flush()
		}
	}
	flush()
	return runs
}

// addOperands flattens a `+` chain into its operands, left to right.
func addOperands(e ast.Expr) []ast.Expr {
	if b, ok := ast.Unparen(e).(*ast.BinaryExpr); ok && b.Op == token.ADD {
		return append(addOperands(b.X), addOperands(b.Y)...)
	}
	return []ast.Expr{e}
}

// isPathJoin reports whether call is filepath.Join or path.Join.
func isPathJoin(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Join" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && (pkg.Name == "filepath" || pkg.Name == "path")
}

// TestNoLayoutLiterals (harness-reads-layout): no string this tool's non-test sources other than layout.go build
// from literals names a layout.env path: a literal, a `+` of adjacent literals, or adjacent literal arguments
// of filepath.Join or path.Join. Comments and import paths are not strings the sensors act on.
func TestNoLayoutLiterals(t *testing.T) {
	paths := layoutPaths(t)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, f := range files {
		if f == "layout.go" || strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		imports := map[*ast.BasicLit]bool{}
		for _, imp := range file.Imports {
			imports[imp.Path] = true
		}
		reported := map[string]bool{} // "<line> <path>": a literal inside a run is reported once
		check := func(pos token.Pos, s string) {
			for _, p := range paths {
				line := fset.Position(pos).Line
				key := strconv.Itoa(line) + " " + strings.Join(p, "/")
				if namesPath(s, p) && !reported[key] {
					reported[key] = true
					t.Errorf("%s:%d: string literal names the template layout path %q: read it from scripts/harness/layout.env through layout.go [harness-reads-layout]", f, line, strings.Join(p, "/"))
				}
			}
		}
		inChain := map[ast.Node]bool{} // inner `+` of a chain already checked from its top
		var markChain func(e ast.Expr)
		markChain = func(e ast.Expr) {
			if b, ok := ast.Unparen(e).(*ast.BinaryExpr); ok && b.Op == token.ADD {
				inChain[b] = true
				markChain(b.X)
				markChain(b.Y)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BasicLit:
				if s, ok := constString(n); ok && !imports[n] {
					check(n.Pos(), s)
				}
			case *ast.BinaryExpr:
				if n.Op != token.ADD || inChain[n] {
					return true
				}
				markChain(n.X)
				markChain(n.Y)
				for _, s := range constRuns(addOperands(n), "") {
					check(n.Pos(), s)
				}
			case *ast.CallExpr:
				if isPathJoin(n) {
					for _, s := range constRuns(n.Args, "/") {
						check(n.Pos(), s)
					}
				}
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("no non-test Go source in the package directory: run go test from apps/api/tools/harness")
	}
}
