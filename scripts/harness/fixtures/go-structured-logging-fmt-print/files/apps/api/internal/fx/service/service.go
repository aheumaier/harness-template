// Package service violates go-structured-logging (forbidigo).
package service

import "fmt"

// Report prints an unstructured line to stdout.
func Report(n int) { fmt.Println("processed", n) }
