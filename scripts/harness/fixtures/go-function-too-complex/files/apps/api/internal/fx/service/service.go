// Package service is a selftest fixture: one function over the complexity limit.
package service

// FixtureTooComplex nests four ifs (1+2+3+4) and adds four flat ones: cognitive complexity 14.
func FixtureTooComplex(a int) int {
	r := 0
	if a > 0 {
		if a > 1 {
			if a > 2 {
				if a > 3 {
					r++
				}
			}
		}
	}
	if a > 4 {
		r++
	}
	if a > 5 {
		r++
	}
	if a > 6 {
		r++
	}
	if a > 7 {
		r++
	}
	return r
}
