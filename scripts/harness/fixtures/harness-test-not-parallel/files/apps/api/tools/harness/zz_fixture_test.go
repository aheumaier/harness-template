package main

import "testing"

func TestFixtureSerial(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("no name")
	}
}
