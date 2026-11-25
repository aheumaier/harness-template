package main

import (
	"testing"
	"time"
)

// TestStopGateHeld: a tree that was held back passes with a message until it changes, also in later turns.
func TestStopGateHeld(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, recipe string
		limit, after time.Duration
	}{
		{name: "red", recipe: "@exit 1", limit: time.Minute, after: time.Minute},
		{name: "red with a tool built meanwhile", recipe: "@echo x >> .bin/tool; exit 1", limit: time.Minute,
			after: time.Minute},
		{name: "not done", recipe: "@sleep 60", limit: 3 * time.Second, after: time.Minute},
		{name: "red without a second key", recipe: "@exit 1", limit: time.Minute, after: time.Nanosecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := gateRepo(t, tt.recipe)
			writeLines(t, root, ".bin/other", "x")
			steps := []struct {
				name   string
				change func()
				block  bool
			}{
				{"first", func() {}, true},
				{"the same tree", func() {}, false},
				{"committed", func() { commitAll(t, root, "more") }, false},
				{"edited", func() { writeLines(t, root, "a.go", "c") }, true},
				{"the same edit", func() {}, false},
				{"a tool changed", func() { writeLines(t, root, ".bin/tool", "changed") }, true},
			}
			runs := 0
			for _, s := range steps {
				s.change()
				msg, err := gate(t, t.Context(), root, firstStop, tt.limit, tt.after)
				if (err != nil) != s.block || (msg == "") != s.block {
					t.Errorf("%s: error %v, systemMessage %q; want a block: %v", s.name, err, msg, s.block)
				}
				if s.block {
					runs++
				}
				if got := checkRuns(t, root); got != runs {
					t.Errorf("%s: make check ran %d times, want %d", s.name, got, runs)
				}
			}
			busy := `{"stop_hook_active":false,"background_tasks":[{"status":"running","type":"subagent"}]}`
			if msg, err := gate(t, t.Context(), root, busy, tt.limit, tt.after); err != nil || msg != "" {
				t.Errorf("busy: error %v, systemMessage %q; want neither", err, msg)
			}
			if err := keyOf(t, root).stamp(root, greenStamp); err != nil {
				t.Fatal(err)
			}
			if msg, err := gate(t, t.Context(), root, firstStop, tt.limit, tt.after); err != nil || msg != "" {
				t.Errorf("green: error %v, systemMessage %q; want neither", err, msg)
			}
		})
	}
}
