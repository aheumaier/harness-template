#!/usr/bin/env bats
# .claude/hooks/guard-bash.sh: blocks force pushes, pushes to main and rm -r/-f outside the repo.
bats_require_minimum_version 1.5.0
load helpers

@test "blocks force pushes and force refspecs" {
  for c in 'git push --force origin feat' 'git push -f origin feat' 'git push --force-with-lease' \
    'git push origin +feat' 'git push --mirror' 'git push origin --delete feat'; do
    run -2 hook guard-bash.sh "$c"
    has 'force'
  done
}

@test "blocks pushes to main or master" {
  for c in 'git push origin main' 'git push origin HEAD:main' 'git push origin feat:refs/heads/master' \
    'FOO=1 git -C . push origin main' 'make check && git push origin main'; do
    run -2 hook guard-bash.sh "$c"
    has 'Push a feature branch'
  done
}

@test "blocks a bare git push from main" {
  git -C "$BATS_TEST_TMPDIR" init -q -b main
  run -2 hook guard-bash.sh 'git push' "$BATS_TEST_TMPDIR"
  has 'pushes main'
}

@test "allows pushes of feature branches" {
  git -C "$BATS_TEST_TMPDIR" init -q -b feat
  for c in 'git push' 'git push -u origin feat' 'git -C . push origin feat:feat'; do
    run -0 hook guard-bash.sh "$c" "$BATS_TEST_TMPDIR"
  done
}

@test "blocks rm -r/-f outside the repo or with unexpanded paths" {
  for c in 'rm -rf /tmp/x' 'rm -rf ../other' 'rm -fr /' 'FOO=1 rm -r /' 'rm -rf "$HOME/x"' 'rm -rf ~' \
    'rm -r `pwd`' 'cd x && rm -rf /etc'; do
    run -2 hook guard-bash.sh "$c"
    has 'guard-bash: blocked: rm'
  done
}

@test "allows rm inside the repo and non-recursive rm" {
  for c in 'rm -rf node_modules dist' 'rm -rf ./build' 'rm file.txt /etc/x' 'ls; git status' 'echo "a; b" | grep x'; do
    run -0 hook guard-bash.sh "$c"
  done
}

@test "ignores heredoc bodies but checks the commands after them" {
  run -0 hook guard-bash.sh $'git commit -F - <<\'EOF\'\nmention rm -rf / and git push --force\nEOF\necho done'
  run -2 hook guard-bash.sh $'cat <<-X >f\n\trm -rf /\n\tX\nrm -rf /tmp/after'
  has '/tmp/after'
}
