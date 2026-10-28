package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// hookInput is the part of Claude Code's hook JSON the hooks read.
type hookInput struct {
	Cwd       string `json:"cwd"`
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"tool_input"`
}

// parseHookInput reads one JSON object; anything else (no object, trailing data, a field of the wrong type)
// is an error, so the hooks fail closed.
func parseHookInput(r io.Reader) (*hookInput, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var in *hookInput
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, fmt.Errorf("cannot parse the hook input: %w", err)
	}
	if in == nil {
		return nil, errors.New("cannot parse the hook input: not a JSON object")
	}
	return in, nil
}

// fields returns the named fields joined by newlines (no trailing one). `target` is the edited file:
// tool_input.file_path, else tool_input.notebook_path. Only the last field may contain newlines.
func (in *hookInput) fields(names []string) (string, error) {
	out := make([]string, len(names))
	for i, n := range names {
		switch n {
		case "cwd":
			out[i] = in.Cwd
		case "tool_name":
			out[i] = in.ToolName
		case "command":
			out[i] = in.ToolInput.Command
		case "target":
			out[i] = in.ToolInput.FilePath
			if out[i] == "" {
				out[i] = in.ToolInput.NotebookPath
			}
		default:
			return "", fmt.Errorf("unknown field %q (%s)", n, usage)
		}
	}
	return strings.Join(out, "\n"), nil
}
