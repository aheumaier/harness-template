// Package service violates go-http-only-in-api.
package service

import "net/http"

// Status returns an HTTP status code from business logic.
func Status() int { return http.StatusTeapot }
