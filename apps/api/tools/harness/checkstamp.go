package main

import (
	"errors"
	"fmt"
)

func init() {
	commands.register(command{name: "check-stamp", usage: "harness check-stamp <root> [key]",
		about: "harness-stop-is-green: without a key prints the tree's; with the key taken before a green `make check` " +
			"stamps the tree green (make check)", run: checkStamp})
}

// checkStamp is the two ends of the line `make check` ends with (ADR 0048). It never turns a green check red: a
// stamp it cannot write costs the next stop a check, and one line on stderr says why.
func checkStamp(w world, args []string) int {
	if len(args) != 1 && len(args) != 2 {
		return exitBadArgs
	}
	if len(args) == 2 && args[1] == "" {
		return 0 // make -i, or the key could not be taken, which was said then
	}
	if err := stampOrPrint(w, args); err != nil {
		fmt.Fprintf(w.stderr, "check-stamp: this tree is not stamped green: %v\n", err)
	}
	return 0
}

func stampOrPrint(w world, args []string) error {
	root, err := w.abs(args[0])
	if err != nil {
		return err
	}
	k, err := w.treeKey(root)
	switch {
	case err != nil:
		return err
	case len(args) == 1:
		_, err = fmt.Fprintln(w.stdout, k.source())
		return err
	case k.source() != args[1]:
		return errors.New("it changed while make check ran")
	}
	return k.stamp(root, greenStamp)
}
