package domainiso_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/aheumaier/harness-template/apps/api/tools/analyzers/domainiso"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), domainiso.Analyzer, "example/internal/foo/service")
}
