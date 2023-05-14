// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockplot

import (
	"math"
)

func getCandleWidth(mX float64, maxBorderWidth int) (candleWidth, lineWidth, borderWidth int) {
	const minCandleWidth = 5
	const minLineWidth = 1
	const defaultCandleMultiplier = 0.8

	/*// There may be a banking holiday or similar at the start of the candle.
	// We want to adjust candle width in this case, but only for weekly candles.
	// TODO this should be optional
	// TODO maybe consider other bank holidays, not just at the beginning of the week.
	candleDurationMultiplier := 1.0
	if r == candles.CandleOneWeek {
		nt := r.GetNthCandleTime(t, 0)
		startDeviationDays := int(t.Sub(nt).Hours() / 24)
		if startDeviationDays > 0 {
			// consider 5 banking days per week
			// this is kind of a hack, but since this code is only relevant
			// for weekly candles, it is fine.
			candleDurationMultiplier -= (float64(startDeviationDays) / 5)
		}
	}
	// Apply duration multiplier twice to achieve constant spacing.
	candleWidth = int(math.Abs(mX) * defaultCandleMultiplier * candleDurationMultiplier * candleDurationMultiplier)*/
	candleWidth = int(math.Abs(mX) * defaultCandleMultiplier)
	if candleWidth < minCandleWidth {
		candleWidth = minCandleWidth
	}
	lineWidth = candleWidth / 16
	if lineWidth < minLineWidth {
		lineWidth = minLineWidth
	}
	borderWidth = candleWidth / minCandleWidth
	if borderWidth > maxBorderWidth {
		borderWidth = maxBorderWidth
	}
	return
}
