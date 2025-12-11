// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package candles

import (
	"strconv"
	"time"
)

type CandleResolution int32

const (
	CandleOneMinute CandleResolution = iota
	CandleFiveMinutes
	CandleFifteenMinutes
	CandleThirtyMinutes
	CandleSixtyMinutes
	CandleOneDay
	CandleOneWeek
	CandleOneMonth
)

const NumCandleResolutions = CandleOneMonth + 1

func CandleResolutionFromString(s string) CandleResolution {
	// Ignore error and return 0 if invalid.
	r, _ := strconv.ParseInt(s, 10, 32)
	return CandleResolution(r)
}

func CandleResolutionUiStringList() []string {
	return []string{
		"1 min",
		"5 min",
		"15 min",
		"30 min",
		"60 min",
		"1 day",
		"1 week",
		"1 month",
	}
}

func (r CandleResolution) FormatString() string {
	switch r {
	case CandleOneMinute:
		return "15:04"
	case CandleFiveMinutes:
		return "15:04"
	case CandleFifteenMinutes:
		return "15:04"
	case CandleThirtyMinutes:
		return "15:04"
	case CandleSixtyMinutes:
		return "15:04"
	case CandleOneDay:
		return "02 Jan 06"
	case CandleOneWeek:
		return "02 Jan 06"
	case CandleOneMonth:
		return "02 Jan 06"
	default:
		panic("unsupported candle resolution")
	}
}

func (r CandleResolution) GetDuration(context time.Time) time.Duration {
	switch r {
	case CandleOneMinute:
		return time.Minute
	case CandleFiveMinutes:
		return time.Minute * 5
	case CandleFifteenMinutes:
		return time.Minute * 15
	case CandleThirtyMinutes:
		return time.Minute * 30
	case CandleSixtyMinutes:
		return time.Hour
	case CandleOneDay:
		return getDayDuration(context)
	case CandleOneWeek:
		d, _ := getWeekDuration(context)
		return d
	case CandleOneMonth:
		d, _ := getMonthDuration(context)
		return d
	default:
		panic("unsupported candle resolution")
	}
}

func (r CandleResolution) GetDeltaCandleCount(candleTime time.Time, tradeTime time.Time) int {
	unitCount := -1
	// Each duration may be different, therefore we loop.
	// Check whether trade is within or after the current candle.
	// TcandleStart <= Ttrade < TnextCandleStart, so candleTime==tradeTime means current candle.
	for candleTime.Before(tradeTime) {
		unitCount++
		candleTime = candleTime.Add(r.GetDuration(candleTime))
	}
	return unitCount
}

func (r CandleResolution) GetNthCandleTime(t time.Time, n int) time.Time {
	// Get 0th candle time first, so that n = 0 works.
	t = r.getRecentCandleStartTime(t)
	if n < 0 {
		for i := 0; i > n; i-- {
			// Go one second back to the previous interval to get the correct duration.
			t = t.Add(-r.GetDuration(t.Add(-time.Second)))
		}
	}
	for i := 0; i < n; i++ {
		t = t.Add(r.GetDuration(t))
	}
	return t
}

func (r CandleResolution) ConvertTimeToCandleUnits(t time.Time) float64 {
	switch r {
	case CandleOneMinute:
		return float64(t.UnixMilli()) / 60000
	case CandleFiveMinutes:
		return float64(t.UnixMilli()) / (60000 * 5)
	case CandleFifteenMinutes:
		return float64(t.UnixMilli()) / (60000 * 15)
	case CandleThirtyMinutes:
		return float64(t.UnixMilli()) / (60000 * 30)
	case CandleSixtyMinutes:
		return float64(t.UnixMilli()) / (60000 * 60)
	case CandleOneDay:
		return float64(t.Unix()) / (60 * 60 * 24)
	case CandleOneWeek:
		// Jan 1 1970 was a Thursday, we need our weeks to start on Monday.
		// So in this case, we adjust the start and use Monday Jan 5 1970.
		s := time.Date(1970, 1, 5, 0, 0, 0, 0, time.UTC)
		numWeeks := int(t.Sub(s).Hours()) / (24 * 7)
		d, firstDay := getWeekDuration(t)
		return float64(numWeeks) + (t.Sub(firstDay).Seconds() / d.Seconds())
	case CandleOneMonth:
		y, m, _ := t.Date()
		numMonths := (y-1970)*12 + (int(m) - 1)
		d, firstDay := getMonthDuration(t)
		return float64(numMonths) + (t.Sub(firstDay).Seconds() / d.Seconds())
	default:
		panic("unsupported candle resolution")
	}
}

func (r CandleResolution) ConvertCandleUnitsToTime(u float64) time.Time {
	switch r {
	case CandleOneMinute:
		return time.UnixMilli(int64(u * 60000))
	case CandleFiveMinutes:
		return time.UnixMilli(int64(u * 60000 * 5))
	case CandleFifteenMinutes:
		return time.UnixMilli(int64(u * 60000 * 15))
	case CandleThirtyMinutes:
		return time.UnixMilli(int64(u * 60000 * 30))
	case CandleSixtyMinutes:
		return time.UnixMilli(int64(u * 60000 * 60))
	case CandleOneDay:
		return time.Unix(int64(u*60*60*24), 0)
	case CandleOneWeek:
		// Jan 1 1970 was a Thursday, we need our weeks to start on Monday.
		// So in this case, we adjust the start and use Monday Jan 5 1970.
		firstDay := time.Date(1970, 1, 5+(int(u)*7), 0, 0, 0, 0, time.UTC)
		d, _ := getWeekDuration(firstDay)
		return firstDay.Add(time.Duration((u - float64(int(u))) * float64(d))).Local()
	case CandleOneMonth:
		firstDay := time.Date(1970, time.Month(1+int(u)), 1, 0, 0, 0, 0, time.UTC)
		d, _ := getMonthDuration(firstDay)
		return firstDay.Add(time.Duration((u - float64(int(u))) * float64(d))).Local()
	default:
		panic("unsupported candle resolution")
	}
}

func (r CandleResolution) getRecentCandleStartTime(t time.Time) time.Time {
	switch r {
	case CandleOneMinute:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
	case CandleFiveMinutes:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()/5*5, 0, 0, t.Location())
	case CandleFifteenMinutes:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()/15*15, 0, 0, t.Location())
	case CandleThirtyMinutes:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()/30*30, 0, 0, t.Location())
	case CandleSixtyMinutes:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case CandleOneDay:
		// We use UTC start of day as normalised start of day-based candles.
		// The broker may use timestamps of closing time, which may even be non-constant.
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	case CandleOneWeek:
		// Candlestick weeks start on Mondays. Golang Weeks start on Sundays.
		// We need to adjust the difference.
		weekdayDiff := int(t.Weekday()) - int(time.Monday)
		if weekdayDiff < 0 {
			weekdayDiff = 7 + weekdayDiff
		}
		return time.Date(t.Year(), t.Month(), t.Day()-weekdayDiff, 0, 0, 0, 0, time.UTC)
	case CandleOneMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		panic("unsupported candle resolution")
	}
}

func getDayDuration(t time.Time) time.Duration {
	y := t.Year()
	m := t.Month()
	d := t.Day()
	return time.Date(y, m, d+1, 0, 0, 0, 0, t.Location()).Sub(
		time.Date(y, m, d, 0, 0, 0, 0, t.Location()),
	)
}

func getWeekDuration(t time.Time) (time.Duration, time.Time) {
	// Candlestick weeks start on Mondays. Golang Weeks start on Sundays.
	// We need to adjust the difference.
	weekdayDiff := int(t.Weekday()) - int(time.Monday)
	if weekdayDiff < 0 {
		weekdayDiff = 7 + weekdayDiff
	}
	y, m, d := t.Date()
	d -= weekdayDiff
	s := time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	return time.Date(y, m, d+7, 0, 0, 0, 0, t.Location()).Sub(s), s
}

func getMonthDuration(t time.Time) (time.Duration, time.Time) {
	// Use "Sub" call so that daylight saving time is considered.
	y := t.Year()
	m := t.Month()
	s := time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
	return time.Date(y, m+1, 1, 0, 0, 0, 0, t.Location()).Sub(s), s
}
