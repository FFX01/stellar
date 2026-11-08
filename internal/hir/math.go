package hir

import "math"

func AddSafe(a, b int) (int, bool) {
	if b > 0 && a > math.MaxInt-b {
		return 0, true
	}
	if b < 0 && a < math.MinInt-b {
		return 0, true
	}
	return a + b, false
}

func SubSafe(a, b int) (int, bool) {
	if b < 0 && a > math.MaxInt+b {
		return 0, true // overflow
	}
	if b > 0 && a < math.MinInt+b {
		return 0, true // underflow
	}
	return a - b, false
}

func MulSafe(a, b int) (int, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	res := a * b
	if res/a != b {
		return 0, true // overflow occurred
	}
	return res, false
}

func DivSafe(a, b int) (int, bool) {
	// 1. Prevent division by zero
	if b == 0 {
		return 0, true // Error: Division by zero
	}

	// 2. Prevent signed overflow (MinInt / -1)
	if a == math.MinInt && b == -1 {
		return 0, true // Error: Overflow
	}

	return floorDiv(a, b), false
}

func floorDiv(a, b int) int {
	q := a / b
	r := a % b
	if r != 0 && ((a ^ b) < 0) {
		q--
	}
	return q
}

func FloorMod(a, b int) int {
	res := a % b
	// If the signs are different and the remainder is not zero,
	// adjust the result by adding the divisor
	if (res < 0 && b > 0) || (res > 0 && b < 0) {
		res += b
	}
	return res
}

func ModSafe(a, b int) (int, bool) {
	if b == 0 {
		return 0, true
	}
	if a == math.MinInt && b == -1 {
		return 0, true
	}
	return FloorMod(a, b), false
}
