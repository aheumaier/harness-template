package main

// fixtureTooComplex nests eight ifs (1+2+...+8 = 36) and adds six flat ones: cognitive complexity 42.
func fixtureTooComplex(a int) int {
	r := 0
	if a > 1 {
		if a > 2 {
			if a > 3 {
				if a > 4 {
					if a > 5 {
						if a > 6 {
							if a > 7 {
								if a > 8 {
									r++
								}
							}
						}
					}
				}
			}
		}
	}
	if a > 9 {
		r++
	}
	if a > 10 {
		r++
	}
	if a > 11 {
		r++
	}
	if a > 12 {
		r++
	}
	if a > 13 {
		r++
	}
	if a > 14 {
		r++
	}
	return r
}
