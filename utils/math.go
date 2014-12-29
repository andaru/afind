package utils

// MinInt returns the minimum of two ints
func MinInt(l, r int) int {
	if l > r {
		return r
	}
	return l
}

// MinInt returns the maximum of two ints
func MaxInt(l, r int) int {
	if l < r {
		return r
	}
	return l
}
