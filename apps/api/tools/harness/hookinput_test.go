package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHookInput(t *testing.T) {
	tests := []struct {
		in     string
		fields []string
		want   string
	}{
		{`{"cwd":"/r","tool_name":"Bash","tool_input":{"command":"ls\nx\n"}}`,
			[]string{"cwd", "tool_name", "target", "command"}, "/r\nBash\n\nls\nx\n"},
		{`{"tool_input":{"file_path":"a.go","notebook_path":"b.ipynb"}}`, []string{"target"}, "a.go"},
		{`{"tool_input":{"notebook_path":"b.ipynb"}}`, []string{"target"}, "b.ipynb"},
		{`{"other":1}`, []string{"command", "cwd"}, "\n"},
		{`{}`, nil, ""},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"hook-input"}, tt.fields...), strings.NewReader(tt.in), &stdout, &stderr); code != 0 {
			t.Errorf("%s: exit %d: %s", tt.in, code, stderr.String())
		}
		if stdout.String() != tt.want {
			t.Errorf("%s %v: got %q, want %q", tt.in, tt.fields, stdout.String(), tt.want)
		}
	}
}

func TestHookInputFailsClosed(t *testing.T) {
	for _, tt := range []struct{ in, field string }{
		{"not json", "command"}, {"", "command"}, {"null", "command"}, {`{"cwd":3}`, "cwd"}, {`{}x`, "cwd"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"hook-input", tt.field}, strings.NewReader(tt.in), &stdout, &stderr); code != 2 {
			t.Errorf("%q %s: exit %d, want 2", tt.in, tt.field, code)
		}
		if stdout.Len() != 0 {
			t.Errorf("%q %s: printed %q", tt.in, tt.field, stdout.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(nil, strings.NewReader(""), &stdout, &stderr); code != exitUsage {
		t.Errorf("no subcommand: exit %d, want %d", code, exitUsage)
	}
	if code := run([]string{"hook-input", "nope"}, strings.NewReader("{}"), &stdout, &stderr); code != exitUsage {
		t.Errorf("unknown field: exit %d, want %d", code, exitUsage)
	}
}
