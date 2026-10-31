#!/usr/bin/env bash
# Runs in the repo copy: one function body pasted into a second function of the same package (dupl compares
# the files of one package). The pair is 29 dupl tokens, just past the threshold of 26 in apps/api/.golangci.yml,
# so the fixture stops reproducing if that threshold is raised past 29.
set -euo pipefail
mkdir -p apps/api/internal/fx/service
body() { # name -> a function that normalises a name, the same body each time
  printf '\n// %s lowers a name and joins its words with dashes.\n' "$1"
  printf 'func %s(name string) string {\n' "$1"
  printf '\ts := strings.ToLower(name)\n'
  printf '\treturn strings.Join(strings.Fields(s), "-")\n}\n'
}
{
  printf '// Package service is a selftest fixture: one function body copied into a second function.\npackage service\n\n'
  printf 'import "strings"\n'
  body NormalizeName
  body NormalizeTag
} >apps/api/internal/fx/service/service.go
