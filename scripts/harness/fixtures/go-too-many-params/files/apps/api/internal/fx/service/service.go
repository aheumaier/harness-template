// Package service is a selftest fixture: one function over the parameter limit.
package service

// FixtureTooManyParams takes five values that should travel as one struct.
func FixtureTooManyParams(a, b, c, d, e int) int { return a + b + c + d + e }
