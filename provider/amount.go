package provider

import "math"

// MinorUnits converts an amount to kuruş/cents. It rounds: a plain int(amount*100) truncates,
// and 19.99*100 is 1998.9999999999998 in float64, which charged or refunded one kuruş less.
func MinorUnits(amount float64) int64 {
	return int64(math.Round(amount * 100))
}
