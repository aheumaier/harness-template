package main

// spec-refs: an OpenAPI document (YAML or JSON) must keep its `$ref`s internal (`#/...`), because oasdiff follows an
// external one (a URL or another file): contract-check.sh (api-no-breaking-change) and upstream-drift.sh
// (api-upstream-no-breaking-change) call it before oasdiff. It parses the document rather than grepping lines, so
// a `$ref` key quoted, spaced before its colon, with its value on the next line, or reached through a YAML alias
// counts too.
//
//	harness spec-refs <file>...
//
// One line per external `$ref`: `<file>:<line>: $ref <value> points outside this file (want '#/...')`. Exit 0
// when there is none, 1 when there is any, 2 when a file cannot be read or parsed.

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// specRefs prints each external `$ref` of the files and reports whether there was any.
func specRefs(files []string, stdout io.Writer) (bool, error) {
	bad := false
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return bad, err
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return bad, fmt.Errorf("%s: %w", f, err)
		}
		for _, r := range externalRefs(&doc) {
			bad = true
			fmt.Fprintf(stdout, "%s:%d: $ref %s points outside this file (want '#/...')\n", f, r.line, r.value)
		}
	}
	return bad, nil
}

type externalRef struct {
	line  int
	value string
}

// externalRefs walks the tree (each node once: an alias is followed to its anchor, which is walked where it is
// defined) and returns every `$ref` whose value is not a string starting with `#`.
func externalRefs(n *yaml.Node) []externalRef {
	var out []externalRef
	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := deref(n.Content[i]), deref(n.Content[i+1])
				if k.Kind == yaml.ScalarNode && k.Value == "$ref" &&
					(v.Kind != yaml.ScalarNode || !strings.HasPrefix(v.Value, "#")) {
					out = append(out, externalRef{line: n.Content[i].Line, value: refValue(v)})
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(n)
	return out
}

// deref follows aliases to the node they name.
func deref(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

// refValue is a `$ref` value for a finding: the scalar quoted, or what kind of node it is.
func refValue(v *yaml.Node) string {
	if v.Kind == yaml.ScalarNode {
		return fmt.Sprintf("%q", v.Value)
	}
	return "(not a string)"
}
