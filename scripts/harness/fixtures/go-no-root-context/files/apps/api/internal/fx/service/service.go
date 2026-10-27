// Package service violates go-no-root-context.
package service

import "context"

// Detached drops the caller's cancellation and trace.
func Detached() context.Context { return context.Background() }
