package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Suppression markers are assembled from parts so this file never contains one itself: it is hand-written
// Go and the sensor reads it like any other.
const (
	nolint      = "//" + "nolint:noctxroot"
	eslint      = "// eslint" + "-disable-next-line no-console"
	shellcheck  = "# shellcheck" + " disable=SC2086"
	tsIgnore    = "@ts" + "-ignore"
	tsNocheck   = "@ts" + "-nocheck"
	jscpdIgnore = "jscpd" + ":ignore"
	jscpdStart  = jscpdIgnore + "-start"
	noReason    = ": suppression without 'reason: … (rule: <id>)' on the same line [harness-suppression-reason]"
)

// integrityRepo makes a git repo whose `main` holds a minimal harness (rules.md, protected-paths.txt), a
// four-line .squawk.toml, a sensor script and a Makefile, both configs. It returns the repo.
func integrityRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitT(t, root, "init", "-q", "-b", "main")
	for p, lines := range map[string][]string{
		"docs/harness/rules.md": {"# Rules", "### go-no-root-context", "### sh-shellcheck", "### web-typecheck"},
		"scripts/harness/protected-paths.txt": {"# Harness files", "*.squawk.toml config", "*.gitattributes config",
			"*tsconfig*.json config", "Makefile config", "scripts/harness/* config", "docs/harness/*"},
		".squawk.toml":           {"# Harness file (protected).", `pg_version = "17.0"`, "# Atlas applies each migration file inside a transaction.", "assume_in_transaction = true"},
		"scripts/harness/run.sh": {"#!/usr/bin/env bash", "set -eu", "echo ok"},
		"Makefile":               {"check:", "\t@echo check"},
		"x.go":                   {"package x"},
		layoutFile:               templateLayout,
	} {
		writeLines(t, root, p, lines...)
	}
	commitAll(t, root, "base")
	return root
}

func gitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@localhost",
		"-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "--no-verify", "-m", msg)
}

func writeLines(t *testing.T, root, p string, lines ...string) {
	t.Helper()
	f := filepath.Join(root, p)
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// add appends lines to a file of the repo, creating it.
func add(t *testing.T, root, p string, lines ...string) {
	t.Helper()
	f := filepath.Join(root, p)
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	h, err := os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if _, err := h.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatal(err)
	}
}

// replace rewrites a file of the repo with old replaced by new.
func replace(t *testing.T, root, p, old, new string) {
	t.Helper()
	f := filepath.Join(root, p)
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte(strings.Replace(string(b), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func clone(t *testing.T, own string) string {
	t.Helper()
	pr := filepath.Join(t.TempDir(), "pr")
	gitT(t, own, "clone", "-q", own, pr)
	return pr
}

func TestIntegrityCheck(t *testing.T) {
	tests := []struct {
		name     string
		change   func(t *testing.T, own string) (root string) // root "" is own
		approved bool
		want     []string // output lines; none means the check passes
	}{
		// The cases of the former scripts/test/integrity-check.bats.
		{"passes when nothing changed", func(*testing.T, string) string { return "" }, false, nil},
		{"flags suppressions without a reason, with path and line", func(t *testing.T, own string) string {
			add(t, own, "x.go", "f() "+nolint)
			add(t, own, "a.ts", eslint) // a.ts and s.sh are untracked: still part of the change
			add(t, own, "s.sh", shellcheck)
			return ""
		}, false, []string{"a.ts:1" + noReason, "s.sh:1" + noReason, "x.go:2" + noReason}},
		{"accepts suppressions with reason and rule id", func(t *testing.T, own string) string {
			add(t, own, "x.go", "f() "+nolint+" // reason: closing a read-only file (rule: go-no-root-context)")
			add(t, own, "s.sh", shellcheck+" # reason: word splitting intended (rule: sh-shellcheck)")
			gitT(t, own, "add", "-A")
			return ""
		}, false, nil},
		{"never accepts ts-ignore, ts-nocheck or jscpd ignore comments, even with a reason", func(t *testing.T, own string) string {
			add(t, own, "a.ts", "// "+tsIgnore+" reason: x (rule: web-typecheck)")
			add(t, own, "b.mts", "// "+tsNocheck+" reason: x (rule: web-typecheck)")
			add(t, own, "c.tsx", "// "+jscpdStart+" reason: x (rule: web-typecheck)")
			gitT(t, own, "add", "-A")
			return ""
		}, false, []string{
			"a.ts:1: " + tsIgnore + " is never allowed [harness-suppression-reason]",
			"b.mts:1: " + tsNocheck + " is never allowed [harness-suppression-reason]",
			"c.tsx:1: " + jscpdIgnore + " is never allowed [harness-suppression-reason]",
		}},
		{"flags SQL, staticcheck, inline eslint and workflow suppressions without a reason", func(t *testing.T, own string) string {
			add(t, own, "m.sql", "-- atlas"+":nolint DS103", "-- squawk"+"-ignore ban-drop-column")
			add(t, own, "y.go", "//lint"+":ignore SA1019 old api", "//lint"+":file-ignore U1000 generated elsewhere")
			add(t, own, "c.jsx", "/* eslint"+" no-console: off */")
			add(t, own, ".github/workflows/w.yml", "      - run: echo $x "+shellcheck)
			return ""
		}, false, []string{".github/workflows/w.yml:1" + noReason, "c.jsx:1" + noReason, "m.sql:1" + noReason,
			"m.sql:2" + noReason, "y.go:1" + noReason, "y.go:2" + noReason}},
		{"flags a reason that names no rule in rules.md", func(t *testing.T, own string) string {
			add(t, own, "x.go", "f() "+nolint+" // reason: trust me (rule: go-lint)")
			return ""
		}, false, []string{"x.go:2: suppression names 'go-lint', not a rule id in docs/harness/rules.md [harness-suppression-reason]"}},
		{"skips the layout's generated code (GENERATED_GLOBS)", func(t *testing.T, own string) string {
			writeLines(t, own, layoutFile, withLayout("GENERATED_GLOBS", "*.gen.go:apps/api/internal/*/store/*")...)
			commitAll(t, own, "a layout without the client's generated code")
			add(t, own, "a.gen.go", nolint)
			add(t, own, "apps/api/internal/billing/store/q.go", nolint)
			add(t, own, "packages/api-client/src/gen/x.ts", eslint)
			return ""
		}, false, []string{"packages/api-client/src/gen/x.ts:1" + noReason}},
		{"reports paths with spaces", func(t *testing.T, own string) string {
			writeLines(t, own, "a b.go", "package y")
			commitAll(t, own, "space")
			add(t, own, "a b.go", "f() "+nolint)
			return ""
		}, false, []string{"a b.go:2" + noReason}},
		{"ignores fixture overlays in the suppression check (a new fixture is still a harness change)", func(t *testing.T, own string) string {
			add(t, own, "scripts/harness/fixtures/f/files/x.go", nolint)
			gitT(t, own, "add", "-A")
			return ""
		}, true, nil},
		{"ignores vendored .specify/ in the suppression check, not extensionless git hooks", func(t *testing.T, own string) string {
			add(t, own, ".specify/scripts/bash/x.sh", shellcheck)
			add(t, own, "scripts/git-hooks/commit-msg", shellcheck)
			gitT(t, own, "add", "-A")
			return ""
		}, true, []string{"scripts/git-hooks/commit-msg:1" + noReason}},
		{"the PR's .gitattributes cannot hide a hunk", func(t *testing.T, own string) string {
			writeLines(t, own, ".gitattributes", "*.go -diff", "*.toml binary")
			add(t, own, "x.go", "f() "+nolint)
			add(t, own, ".squawk.toml", `excluded_rules = ["ban-drop-column"]`)
			return ""
		}, false, []string{
			"x.go:2" + noReason,
			".gitattributes:1: added harness config line: *.go -diff [harness-config-not-loosened]",
			".gitattributes:2: added harness config line: *.toml binary [harness-config-not-loosened]",
			`.squawk.toml:5: added harness config line: excluded_rules = ["ban-drop-column"] [harness-config-not-loosened]`,
		}},
		{"flags a change to sensor code", func(t *testing.T, own string) string {
			add(t, own, "scripts/harness/run.sh", "exit 0")
			return ""
		}, false, []string{"scripts/harness/run.sh:4: added harness config line: exit 0 [harness-config-not-loosened]"}},
		{"flags a removed or loosening line in a harness config", func(t *testing.T, own string) string {
			replace(t, own, ".squawk.toml", "assume_in_transaction = true", `excluded_rules = ["require-timeout-settings"]`)
			return ""
		}, false, []string{
			".squawk.toml:4: removed harness config line: assume_in_transaction = true [harness-config-not-loosened]",
			`.squawk.toml:4: added harness config line: excluded_rules = ["require-timeout-settings"] [harness-config-not-loosened]`,
		}},
		{"allows comment-only changes to a harness config", func(t *testing.T, own string) string {
			replace(t, own, ".squawk.toml", "# Atlas applies each migration file inside a transaction.", "# reworded comment")
			return ""
		}, false, nil},
		{"flags a tightening line too: any config change is a human decision", func(t *testing.T, own string) string {
			add(t, own, ".squawk.toml", "pg_version_strict = true")
			return ""
		}, false, []string{".squawk.toml:5: added harness config line: pg_version_strict = true [harness-config-not-loosened]"}},
		{"judges HARNESS_ROOT with its own protected-paths.txt, so a change cannot unprotect itself", func(t *testing.T, own string) string {
			pr := clone(t, own)
			replace(t, pr, "scripts/harness/protected-paths.txt", "*.squawk.toml config\n", "")
			add(t, pr, ".squawk.toml", `excluded_rules = ["ban-drop-column"]`)
			return pr
		}, false, []string{
			`.squawk.toml:5: added harness config line: excluded_rules = ["ban-drop-column"] [harness-config-not-loosened]`,
			"scripts/harness/protected-paths.txt:2: removed harness config line: *.squawk.toml config [harness-config-not-loosened]",
		}},
		{"HARNESS_CHANGE_APPROVED waives the config check, not the suppression check", func(t *testing.T, own string) string {
			add(t, own, ".squawk.toml", `excluded_rules = ["ban-drop-column"]`)
			add(t, own, "x.go", nolint)
			return ""
		}, true, []string{"x.go:2" + noReason}},
		{"a // or /* line in a config is not a comment: it can comment a setting out", func(t *testing.T, own string) string {
			writeLines(t, own, "apps/web/tsconfig.json", "{", `  "strict": true`, "}")
			commitAll(t, own, "ts")
			writeLines(t, own, "apps/web/tsconfig.json", "{", "  /*", `  "strict": true`, "  // */", "}")
			return ""
		}, false, []string{
			"apps/web/tsconfig.json:2: added harness config line:   /* [harness-config-not-loosened]",
			"apps/web/tsconfig.json:4: added harness config line:   // */ [harness-config-not-loosened]",
		}},
		{"looks rule ids up in its own rules.md, not the change's", func(t *testing.T, own string) string {
			pr := clone(t, own)
			add(t, pr, "docs/harness/rules.md", "### made-up-rule")
			add(t, pr, "x.go", "f() "+nolint+" // reason: trust me (rule: made-up-rule)")
			return pr
		}, true, []string{"x.go:2: suppression names 'made-up-rule', not a rule id in docs/harness/rules.md [harness-suppression-reason]"}},
		// What the shell version did not cover.
		{"a deleted harness config reports its removed lines (+++ /dev/null)", func(t *testing.T, own string) string {
			gitT(t, own, "rm", "-q", "scripts/harness/run.sh")
			return ""
		}, false, []string{
			"scripts/harness/run.sh:1: removed harness config line: #!/usr/bin/env bash [harness-config-not-loosened]",
			"scripts/harness/run.sh:2: removed harness config line: set -eu [harness-config-not-loosened]",
			"scripts/harness/run.sh:3: removed harness config line: echo ok [harness-config-not-loosened]",
		}},
		{"a changed shebang is a config line, a comment on line 1 too", func(t *testing.T, own string) string {
			replace(t, own, "scripts/harness/run.sh", "#!/usr/bin/env bash", "#!/usr/bin/true")
			return ""
		}, false, []string{
			"scripts/harness/run.sh:1: removed harness config line: #!/usr/bin/env bash [harness-config-not-loosened]",
			"scripts/harness/run.sh:1: added harness config line: #!/usr/bin/true [harness-config-not-loosened]",
		}},
		{"a file mode change of a harness config is reported (chmod -x on a hook)", func(t *testing.T, own string) string {
			if err := os.Chmod(filepath.Join(own, "scripts/harness/run.sh"), 0o755); err != nil {
				t.Fatal(err)
			}
			return ""
		}, false, []string{"scripts/harness/run.sh: file mode changed from 100644 to 100755 [harness-config-not-loosened]"}},
		{"removing a suppression is not a finding", func(t *testing.T, own string) string {
			add(t, own, "x.go", "f() "+nolint)
			commitAll(t, own, "suppressed")
			replace(t, own, "x.go", "f() "+nolint+"\n", "")
			return ""
		}, false, nil},
		{"a Makefile recipe line is reported without its tab", func(t *testing.T, own string) string {
			add(t, own, "Makefile", "\t@true")
			return ""
		}, false, []string{"Makefile:3: added harness config line: @true [harness-config-not-loosened]"}},
		{"a path git quotes is reported unquoted", func(t *testing.T, own string) string {
			writeLines(t, own, `q"uote.go`, "package q")
			commitAll(t, own, "quote")
			add(t, own, `q"uote.go`, nolint)
			return ""
		}, false, []string{`q"uote.go:2` + noReason}},
		{"committed and staged changes count, not only the working tree", func(t *testing.T, own string) string {
			gitT(t, own, "checkout", "-q", "-b", "feat")
			add(t, own, "x.go", nolint)
			commitAll(t, own, "feat")
			add(t, own, "z.go", nolint)
			gitT(t, own, "add", "z.go")
			return ""
		}, false, []string{"x.go:2" + noReason, "z.go:1" + noReason}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			own := integrityRepo(t)
			root := tt.change(t, own)
			if root == "" {
				root = own
			}
			var out bytes.Buffer
			bad, err := integrityCheck(own, root, "main", tt.approved, &out)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Join(tt.want, "\n")
			if len(tt.want) > 0 {
				want += "\n"
			}
			if bad != (len(tt.want) > 0) || out.String() != want {
				t.Errorf("bad=%v, output:\n%s\nwant:\n%s", bad, out.String(), want)
			}
		})
	}
}

func TestIntegrityCheckFailsClosed(t *testing.T) {
	t.Run("BASE_REF does not resolve", func(t *testing.T) {
		own := integrityRepo(t)
		_, err := integrityCheck(own, own, "no-such-branch", false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "BASE_REF 'no-such-branch' is not a commit") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("no config entries", func(t *testing.T) {
		own := integrityRepo(t)
		writeLines(t, own, "scripts/harness/protected-paths.txt", "# emptied")
		_, err := integrityCheck(own, own, "main", true, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "refusing to pass") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("no rules.md", func(t *testing.T) {
		own := integrityRepo(t)
		if err := os.Remove(filepath.Join(own, "docs/harness/rules.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := integrityCheck(own, own, "main", true, &bytes.Buffer{}); err == nil {
			t.Error("passed without rules.md")
		}
	})
	t.Run("no layout.env", func(t *testing.T) {
		own := integrityRepo(t)
		if err := os.Remove(filepath.Join(own, layoutFile)); err != nil {
			t.Fatal(err)
		}
		if _, err := integrityCheck(own, own, "main", false, &bytes.Buffer{}); err == nil {
			t.Error("passed without layout.env")
		}
	})
	t.Run("no merge base", func(t *testing.T) {
		own := integrityRepo(t)
		gitT(t, own, "checkout", "-q", "--orphan", "other")
		commitAll(t, own, "unrelated")
		_, err := integrityCheck(own, own, "main", false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "have no merge base") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestIntegrityCheckExitCodes(t *testing.T) {
	clean := integrityRepo(t)
	dirty := integrityRepo(t)
	add(t, dirty, "x.go", nolint)
	// origin/main is the clean base; local main has the finding committed, so only origin/main finds it.
	withOrigin := clone(t, clean)
	add(t, withOrigin, "x.go", nolint)
	commitAll(t, withOrigin, "finding")
	config := integrityRepo(t)
	add(t, config, ".squawk.toml", "pg_version_strict = true")
	for _, tt := range []struct {
		name               string
		args               []string
		root, base, waived string
		want               int
	}{
		{"clean", []string{"integrity-check", clean}, "", "main", "", 0},
		{"finding", []string{"integrity-check", dirty}, "", "main", "", 1},
		{"default BASE_REF is main without origin/main", []string{"integrity-check", dirty}, "", "", "", 1},
		{"default BASE_REF is origin/main", []string{"integrity-check", clean}, withOrigin, "", "", 1},
		{"HARNESS_ROOT is read", []string{"integrity-check", clean}, dirty, "main", "", 1},
		{"config change", []string{"integrity-check", config}, "", "main", "", 1},
		{"HARNESS_CHANGE_APPROVED=1 waives it", []string{"integrity-check", config}, "", "main", "1", 0},
		{"only 1 waives it", []string{"integrity-check", config}, "", "main", "true", 1},
		{"base does not resolve", []string{"integrity-check", clean}, "", "nope", "", 2},
		{"no args", []string{"integrity-check"}, "", "", "", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HARNESS_ROOT", tt.root)
			t.Setenv("BASE_REF", tt.base)
			t.Setenv("HARNESS_CHANGE_APPROVED", tt.waived)
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, strings.NewReader(""), &stdout, &stderr); code != tt.want {
				t.Errorf("exit %d, want %d (%s%s)", code, tt.want, stdout.String(), stderr.String())
			}
		})
	}
}
