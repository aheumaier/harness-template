// Package analyzers registers the repo's custom go/analysis linters as a golangci-lint module plugin.
// Add a new analyzer as a sub-package and register it here; wire it in .golangci.yml under linters.settings.custom.
package analyzers

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/aheumaier/harness-template/apps/api/tools/analyzers/domainiso"
	"github.com/aheumaier/harness-template/apps/api/tools/analyzers/noctxroot"
)

func init() {
	register.Plugin("noctxroot", newPlugin(noctxroot.Analyzer))
	register.Plugin("domainiso", newPlugin(domainiso.Analyzer))
}

type plugin struct{ analyzers []*analysis.Analyzer }

func newPlugin(as ...*analysis.Analyzer) register.NewPlugin {
	return func(any) (register.LinterPlugin, error) { return plugin{as}, nil }
}

func (p plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) { return p.analyzers, nil }
func (plugin) GetLoadMode() string                             { return register.LoadModeTypesInfo }
