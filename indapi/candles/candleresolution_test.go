// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package candles

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetMonthDuration(t *testing.T) {
	// December has 31 days
	d, _ := getMonthDuration(time.Date(2022, 12, 24, 10, 10, 10, 0, time.UTC))
	assert.Equal(t, float64(44640), d.Minutes())
	// June has 30 days
	d, _ = getMonthDuration(time.Date(2022, 6, 24, 10, 10, 10, 0, time.UTC))
	assert.Equal(t, float64(43200), d.Minutes())
}

func TestGetWeekDuration(t *testing.T) {
	// Normal week.
	d, _ := getWeekDuration(time.Date(2022, 12, 7, 10, 10, 10, 0, time.UTC))
	assert.Equal(t, float64(10080), d.Minutes())
	// Week with DST
	loc, err := time.LoadLocation("Europe/Berlin")
	assert.NoError(t, err)
	dst := time.Date(2022, 10, 29, 10, 10, 10, 0, loc)
	assert.True(t, dst.IsDST()) // This should use daylight saving time.
	d, _ = getWeekDuration(dst)
	assert.Equal(t, float64(10140), d.Minutes())
}

func TestGetDayDuration(t *testing.T) {
	// Normal day.
	d := getDayDuration(time.Date(2022, 12, 6, 10, 10, 10, 0, time.UTC))
	assert.Equal(t, float64(1440), d.Minutes())
	// Day with DST
	loc, err := time.LoadLocation("Europe/Berlin")
	assert.NoError(t, err)
	dst := time.Date(2022, 10, 30, 10, 10, 10, 0, loc)
	d = getDayDuration(dst)
	assert.Equal(t, float64(1500), d.Minutes())
}

func TestGetNthCandleTime(t *testing.T) {
	r := CandleOneMonth
	d := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	n := r.GetNthCandleTime(d, 1)
	assert.True(t, n.Equal(time.Date(2022, 2, 1, 0, 0, 0, 0, time.UTC)))
	n = r.GetNthCandleTime(d, 12)
	assert.True(t, n.Equal(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))
	n = r.GetNthCandleTime(d, -1)
	assert.True(t, n.Equal(time.Date(2021, 12, 1, 0, 0, 0, 0, time.UTC)))
	n = r.GetNthCandleTime(d, -12)
	assert.True(t, n.Equal(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)))
}

func TestGetZerothCandleTime(t *testing.T) {
	r := CandleOneMonth
	d := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	n := r.GetNthCandleTime(d, 0)
	assert.True(t, n.Equal(d))
	r = CandleOneWeek
	d = time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)
	n = r.GetNthCandleTime(d, 0)
	assert.True(t, n.Equal(d))
	d2 := time.Date(2022, 1, 5, 0, 0, 0, 0, time.UTC)
	n = r.GetNthCandleTime(d2, 0)
	assert.True(t, n.Equal(d))
}

func TestConvertTimeToCandleUnitsWeek(t *testing.T) {
	r := CandleOneWeek
	// Use a Monday first (start of week considering stock candles)
	d := time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)
	n := r.ConvertTimeToCandleUnits(d)
	assert.Equal(t, float64(2713), n)
	// Middle of week is at noon on Thursday
	d = time.Date(2022, 1, 6, 12, 0, 0, 0, time.UTC)
	n = r.ConvertTimeToCandleUnits(d)
	assert.Equal(t, float64(2713.5), n)
}

func TestConvertCandleUnitsToTimeWeek(t *testing.T) {
	r := CandleOneWeek
	n := r.ConvertCandleUnitsToTime(2713)
	assert.True(t, n.Equal(time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)))
	n = r.ConvertCandleUnitsToTime(2713.5)
	assert.True(t, n.Equal(time.Date(2022, 1, 6, 12, 0, 0, 0, time.UTC)))
}

func TestConvertTimeToCandleUnitsMonth(t *testing.T) {
	r := CandleOneMonth
	d := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	n := r.ConvertTimeToCandleUnits(d)
	assert.Equal(t, float64(624), n)
	d = time.Date(2022, 3, 16, 12, 0, 0, 0, time.UTC)
	n = r.ConvertTimeToCandleUnits(d)
	assert.Equal(t, float64(626.5), n)
}

func TestConvertCandleUnitsToTimeMonth(t *testing.T) {
	r := CandleOneMonth
	n := r.ConvertCandleUnitsToTime(624)
	assert.True(t, n.Equal(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)))
	n = r.ConvertCandleUnitsToTime(626.5)
	assert.True(t, n.Equal(time.Date(2022, 3, 16, 12, 0, 0, 0, time.UTC)))
}
