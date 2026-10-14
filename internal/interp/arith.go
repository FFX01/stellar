package interp

import (
	"math"

	"stellar/internal/token"
)

type signed interface {
	~int8 | ~int16 | ~int32 | ~int64
}

type unsigned interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64
}

type float interface{ ~float32 | ~float64 }

// arith applies a binary arithmetic operator to two values of the same
// primitive type. Integer overflow and division by zero panic.
func (in *Interp) arith(op token.Kind, a, b Value, pos token.Pos) Value {
	switch a := a.(type) {
	case int8:
		return signedOp(in, op, a, b.(int8), pos)
	case int16:
		return signedOp(in, op, a, b.(int16), pos)
	case int32:
		return signedOp(in, op, a, b.(int32), pos)
	case int64:
		return signedOp(in, op, a, b.(int64), pos)
	case uint8:
		return unsignedOp(in, op, a, b.(uint8), pos)
	case uint16:
		return unsignedOp(in, op, a, b.(uint16), pos)
	case uint32:
		return unsignedOp(in, op, a, b.(uint32), pos)
	case uint64:
		return unsignedOp(in, op, a, b.(uint64), pos)
	case float32:
		return floatOp(op, a, b.(float32))
	case float64:
		return floatOp(op, a, b.(float64))
	case string:
		return a + b.(string)
	}
	panic("interp: arithmetic on non-numeric value")
}

func signedOp[T signed](in *Interp, op token.Kind, a, b T, pos token.Pos) T {
	var r T
	switch op {
	case token.PLUS:
		r = a + b
		if (b > 0 && r < a) || (b < 0 && r > a) {
			in.overflow(pos)
		}
	case token.MINUS:
		r = a - b
		if (b > 0 && r > a) || (b < 0 && r < a) {
			in.overflow(pos)
		}
	case token.STAR:
		r = a * b
		// r/a recovers b unless the product overflowed. The exception is
		// -1 * min, where the division overflows back to min; min is the
		// only nonzero value equal to its own negation.
		if (a != 0 && r/a != b) || (a == -1 && b != 0 && b == -b) {
			in.overflow(pos)
		}
	case token.SLASH, token.PERCENT:
		if b == 0 {
			in.fail(pos, "division by zero")
		}
		if b == -1 && a != 0 && a == -a { // minimum value / -1
			if op == token.SLASH {
				in.overflow(pos)
			}
			return 0
		}
		if op == token.SLASH {
			r = a / b
		} else {
			r = a % b
		}
	}
	return r
}

func unsignedOp[T unsigned](in *Interp, op token.Kind, a, b T, pos token.Pos) T {
	var r T
	switch op {
	case token.PLUS:
		r = a + b
		if r < a {
			in.overflow(pos)
		}
	case token.MINUS:
		if b > a {
			in.overflow(pos)
		}
		r = a - b
	case token.STAR:
		r = a * b
		if a != 0 && r/a != b {
			in.overflow(pos)
		}
	case token.SLASH, token.PERCENT:
		if b == 0 {
			in.fail(pos, "division by zero")
		}
		if op == token.SLASH {
			r = a / b
		} else {
			r = a % b
		}
	}
	return r
}

func floatOp[T float](op token.Kind, a, b T) T {
	switch op {
	case token.PLUS:
		return a + b
	case token.MINUS:
		return a - b
	case token.STAR:
		return a * b
	}
	return a / b
}

// negate applies unary minus.
func (in *Interp) negate(v Value, pos token.Pos) Value {
	switch v := v.(type) {
	case int8:
		return signedOp(in, token.MINUS, 0, v, pos)
	case int16:
		return signedOp(in, token.MINUS, 0, v, pos)
	case int32:
		return signedOp(in, token.MINUS, 0, v, pos)
	case int64:
		return signedOp(in, token.MINUS, 0, v, pos)
	case float32:
		return -v
	case float64:
		return -v
	}
	panic("interp: negating non-numeric value")
}

// compare orders two primitives of the same type: -1, 0 or 1.
func compare(a, b Value) int {
	switch a := a.(type) {
	case string:
		return cmp3(a, b.(string))
	case float32:
		return cmp3(a, b.(float32))
	case float64:
		return cmp3(a, b.(float64))
	case int8:
		return cmp3(a, b.(int8))
	case int16:
		return cmp3(a, b.(int16))
	case int32:
		return cmp3(a, b.(int32))
	case int64:
		return cmp3(a, b.(int64))
	case uint8:
		return cmp3(a, b.(uint8))
	case uint16:
		return cmp3(a, b.(uint16))
	case uint32:
		return cmp3(a, b.(uint32))
	case uint64:
		return cmp3(a, b.(uint64))
	}
	panic("interp: ordering non-ordered value")
}

func cmp3[T ~string | signed | unsigned | float](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// toInt64 and friends support conversions between numeric types.
func asFloat(v Value) (float64, bool) {
	switch v := v.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

func asInt(v Value) (int64, uint64, bool) {
	switch v := v.(type) {
	case int8:
		return int64(v), 0, true
	case int16:
		return int64(v), 0, true
	case int32:
		return int64(v), 0, true
	case int64:
		return v, 0, true
	case uint8:
		return 0, uint64(v), false
	case uint16:
		return 0, uint64(v), false
	case uint32:
		return 0, uint64(v), false
	case uint64:
		return 0, v, false
	}
	return 0, 0, false
}

// fitsFloat reports whether f, truncated, fits in [lo, hi].
func fitsFloat(f, lo, hi float64) bool {
	return !math.IsNaN(f) && math.Trunc(f) >= lo && math.Trunc(f) <= hi
}
