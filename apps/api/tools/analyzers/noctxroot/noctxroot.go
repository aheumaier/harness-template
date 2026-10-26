// Package noctxroot forbids creating root contexts outside process entry points.
//
// Rule (docs/harness/rules.md#go-no-root-context): context.Background() and context.TODO()
// may only appear under a cmd/ directory or in _test.go files. Everywhere else a context must
// be received from the caller so cancellation, deadlines and trace spans propagate.
package noctxroot

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const message = "context.%s() is only allowed in cmd/ and _test.go [go-no-root-context]. " +
	"Fix: accept `ctx context.Context` as the first parameter and pass it down; " +
	"in an HTTP handler use r.Context(); for work that must outlive the request use context.WithoutCancel(ctx). " +
	"See docs/harness/rules.md#go-no-root-context"

// Analyzer reports root-context construction outside allowed locations.
var Analyzer = &analysis.Analyzer{
	Name:     "noctxroot",
	Doc:      "forbid context.Background/TODO outside cmd/ and tests",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func allowed(filename string) bool {
	slash := filepath.ToSlash(filename)
	return strings.HasSuffix(slash, "_test.go") || strings.Contains(slash, "/cmd/") || strings.HasPrefix(slash, "cmd/")
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ins.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call := n.(*ast.CallExpr)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
		if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "context" {
			return
		}
		if name := fn.Name(); name == "Background" || name == "TODO" {
			if !allowed(pass.Fset.Position(call.Pos()).Filename) {
				pass.Reportf(call.Pos(), message, name)
			}
		}
	})
	return nil, nil
}
