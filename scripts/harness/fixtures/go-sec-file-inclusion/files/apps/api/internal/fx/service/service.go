// Package service violates go-sec (gosec G304).
package service

import "os"

// Load reads a file at a caller-chosen path.
func Load(path string) ([]byte, error) { return os.ReadFile(path) }
