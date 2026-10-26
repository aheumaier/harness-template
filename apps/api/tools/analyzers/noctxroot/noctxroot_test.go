package noctxroot_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/aheumaier/harness-template/apps/api/tools/analyzers/noctxroot"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), noctxroot.Analyzer, "example/service", "example/cmd/app")
}
