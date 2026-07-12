package apiconnect

import "math"

// toInt32 clamps n into the int32 range before converting. Proto's int32
// fields here (Year, Width, Height, Priority, RuntimeSeconds) are always
// small in practice, but an explicit bounds check — rather than a bare
// conversion — is what actually rules out the overflow gosec's G115 flags,
// instead of just silencing the warning.
func toInt32(n int) int32 {
	switch {
	case n > math.MaxInt32:
		return math.MaxInt32
	case n < math.MinInt32:
		return math.MinInt32
	default:
		return int32(n)
	}
}
