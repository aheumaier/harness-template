package main

// A subcommand that no Makefile target, workflow, hook or script runs (fixture repo-orphan-subcommand).
func init() {
	commands.register(rootSensor("zz-orphan", "nothing runs this", func(world, string) (bool, error) { return false, nil }))
}
