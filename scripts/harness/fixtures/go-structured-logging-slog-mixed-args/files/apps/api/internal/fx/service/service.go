// Package service violates go-structured-logging (sloglint).
package service

import "log/slog"

// Report mixes a key-value pair and a slog.Attr in one call.
func Report(l *slog.Logger, n int) { l.Info("processed", "count", n, slog.String("unit", "rows")) }
