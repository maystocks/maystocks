// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockval

import (
	"strconv"

	"github.com/ericlagergren/decimal"
)

const NearZero = 0.000001

// Returns a new decimal containing the delta percentage value.
func CalculateDeltaPercentage(baseValue, currentValue *decimal.Big) *decimal.Big {
	percentage := new(decimal.Big)
	// Check for non-zero, see https://github.com/ericlagergren/decimal/pull/157
	if baseValue.Sign() != 0 {
		percentage.Quo(currentValue, baseValue)
		percentage.Sub(percentage, decimal.New(1, 0))
		percentage.Mul(percentage, decimal.New(100, 0))
	}
	return percentage
}

// RoundPrice rounds price z to two digits after decimal point and returns z.
func RoundPrice(z *decimal.Big) *decimal.Big {
	// Call Quantize twice, otherwise one digit may be missing, see https://github.com/ericlagergren/decimal/issues/151
	return z.Quantize(2).Quantize(2)
}

// RoundPrice rounds percentage z to two digits after decimal point and returns z.
func RoundPercentage(z *decimal.Big) *decimal.Big {
	// Call Quantize twice, otherwise one digit may be missing, see https://github.com/ericlagergren/decimal/issues/151
	return z.Quantize(2).Quantize(2)
}

// Returns a new decimal with prepared formatting, enforce a minimum of 2 digits after decimal point.
func PrepareFormattedPrice(z *decimal.Big) *decimal.Big {
	if z.Scale() < 2 {
		// Adding 0.00 will enforce the proper format
		return new(decimal.Big).Add(z, decimal.New(0, 2))
	}
	return new(decimal.Big).Copy(z)
}

// The builtin decimal.Big conversion from float64 is an "exact" conversion, and useless for our cases.
// Therefore, convert using string conversion, even though this requires memory allocation.
// See also https://github.com/ericlagergren/decimal/issues/142

// Convert float to string and then to decimal.
func ConvertFloatToDecimal(v float64, bitSize int) *decimal.Big {
	d, _ := new(decimal.Big).SetString(strconv.FormatFloat(v, 'f', -1, bitSize))
	return d
}

// Calculate the number of segments for a plot grid
func CalcNumSegments(pos int, margin int, grid int) int {
	if grid == 0 {
		return 0
	}
	return max((pos-margin+grid)/grid, 0)
}

func IsGreenCandle(o, c float64) bool {
	// this may be adjusted based on whether it is considered to be green if open price equals close price.
	return c >= o
}

func IsGreenQuote(percentage *decimal.Big) bool {
	return percentage != nil && !percentage.Signbit()
}

func IsGreaterThanZero(v *decimal.Big) bool {
	return v != nil && v.CmpTotal(new(decimal.Big)) > 0
}
