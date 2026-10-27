#!/usr/bin/env bats
# .claude/hooks/guard-bash.sh: blocks force pushes, pushes to main, rm -r outside the repo and .env access.
bats_require_minimum_version 1.5.0
load helpers

@test "blocks force pushes and force refspecs" {
  for c in 'git push --force origin feat' 'git push -f origin feat' 'git push --force-with-lease' \
    'git push origin +feat' 'git push --mirror' 'git push origin --delete feat' 'git push -fu origin feat' \
    $'git push \\\n  --force origin feat'; do
    run -2 hook guard-bash.sh "$c"
    has 'force'
  done
}

@test "blocks pushes to main or master" {
  for c in 'git push origin main' 'git push origin HEAD:main' 'git push origin feat:refs/heads/master' \
    'FOO=1 git -C . push origin main' 'make check && git push origin main' 'git push origin "main"' \
    "git push origin 'main'"; do
    run -2 hook guard-bash.sh "$c"
    has 'Push a feature branch'
  done
}

@test "blocks a bare git push and HEAD/@ refspecs from main" {
  git -C "$BATS_TEST_TMPDIR" init -q -b main
  for c in 'git push' 'git push -u origin HEAD' 'git push origin @'; do
    run -2 hook guard-bash.sh "$c" "$BATS_TEST_TMPDIR"
    has 'main'
  done
}

@test "allows pushes of feature branches" {
  git -C "$BATS_TEST_TMPDIR" init -q -b feat
  for c in 'git push' 'git push -u origin feat' 'git -C . push origin feat:feat' 'git push -u origin HEAD'; do
    run -0 hook guard-bash.sh "$c" "$BATS_TEST_TMPDIR"
  done
}

@test "blocks rm -r outside the repo or with unexpanded paths" {
  for c in 'rm -rf /tmp/x' 'rm -rf ../other' 'rm -fr /' 'FOO=1 rm -r /' 'rm -rf "$HOME/x"' 'rm -rf ~' \
    'rm -r `pwd`' 'cd x && rm -rf /etc' 'rm -rf .*' 'find . | xargs rm -rf' $'rm -rf \\\n/tmp/x'; do
    run -2 hook guard-bash.sh "$c"
    has 'guard-bash: blocked: rm'
  done
}

@test "sees rm behind wrappers, keywords, grouping and quotes" {
  for c in '(rm -rf /)' '{ rm -rf /; }' 'echo $(rm -rf /x)' '\rm -rf /x' '"rm" -rf /x' 'sudo -n rm -rf /x' \
    'if true; then rm -rf /tmp/a; fi' 'for d in a; do rm -rf /x/$d; done' 'env -i rm -rf /x' 'time rm -r /x'; do
    run -2 hook guard-bash.sh "$c"
    has 'guard-bash: blocked: rm'
  done
}

@test "allows rm inside the repo and non-recursive rm" {
  for c in 'rm -rf node_modules dist' 'rm -rf ./build' 'rm -rf dist/*' 'rm file.txt /etc/x' 'rm -f /tmp/x.log' \
    'ls; git status' 'echo "a; b" | grep x' "rm -rf $REPO/apps/web/dist"; do
    run -0 hook guard-bash.sh "$c"
  done
}

@test "blocks commands that name a .env file, except .env.example" {
  for c in 'cat .env.local' 'grep URL .env.local' 'cp .env.local /tmp/x' 'source ./.env' 'wc -l <.env.local' \
    'cat apps/api/.env'; do
    run -2 hook guard-bash.sh "$c"
    has '.env files hold'
  done
  run -0 hook guard-bash.sh 'cat .env.example'
}

@test "ignores heredoc bodies but checks the commands after them" {
  run -0 hook guard-bash.sh $'git commit -F - <<\'EOF\'\nrm -rf /\ngit push --force\nEOF\necho done'
  run -2 hook guard-bash.sh $'cat <<-X >f\n\trm -rf /\n\tX\nrm -rf /tmp/after'
  has '/tmp/after'
}

@test "a here-string or an unterminated heredoc does not hide later commands" {
  run -2 hook guard-bash.sh $'grep x <<<foo\nrm -rf /tmp/x'
  has '/tmp/x'
  run -2 hook guard-bash.sh $'echo "<<EOF"\nrm -rf /tmp/y'
  has '/tmp/y'
}

@test "fails closed on unparseable input" {
  run -2 "$REPO/.claude/hooks/guard-bash.sh" <<<'not json'
  has 'cannot parse'
}
