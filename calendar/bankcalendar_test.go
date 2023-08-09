// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsBankHoliday2023(t *testing.T) {
	c := NewUSBankCalendar()
	isHoliday, _ := c.IsBankHoliday(time.Date(2023, 1, 1, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)
	// observed holiday
	isHoliday, name := c.IsBankHoliday(time.Date(2023, 1, 2, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)
	assert.True(t, strings.HasSuffix(name, observedHolidayPostfix))

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 1, 16, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 2, 20, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 5, 29, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 6, 19, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 7, 4, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 9, 4, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 10, 9, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 11, 11, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)
	// observed holiday
	isHoliday, name = c.IsBankHoliday(time.Date(2023, 11, 10, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)
	assert.True(t, strings.HasSuffix(name, observedHolidayPostfix))

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 11, 23, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)

	isHoliday, _ = c.IsBankHoliday(time.Date(2023, 12, 25, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, isHoliday)
}

func TestIsTradingDayPartial(t *testing.T) {
	c := NewUSBankCalendar()
	trading, partial := c.IsTradingDay(time.Date(2023, 7, 3, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.True(t, partial)

	trading, partial = c.IsTradingDay(time.Date(2023, 11, 24, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.True(t, partial)

	// Christmas eve is Sunday in 2023, no trading
	trading, partial = c.IsTradingDay(time.Date(2023, 12, 24, 0, 0, 0, 0, c.bankLocation))
	assert.False(t, trading)
	assert.False(t, partial)

	// Christmas eve is Saturday in 2022, no trading
	trading, partial = c.IsTradingDay(time.Date(2022, 12, 24, 0, 0, 0, 0, c.bankLocation))
	assert.False(t, trading)
	assert.False(t, partial)

	// Christmas eve is Friday in 2021, but observed holiday, no trading
	trading, partial = c.IsTradingDay(time.Date(2021, 12, 24, 0, 0, 0, 0, c.bankLocation))
	assert.False(t, trading)
	assert.False(t, partial)

	trading, partial = c.IsTradingDay(time.Date(2020, 12, 24, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.True(t, partial)
}

func TestIsTradingDayWeekday(t *testing.T) {
	c := NewUSBankCalendar()
	trading, partial := c.IsTradingDay(time.Date(2023, 8, 5, 0, 0, 0, 0, c.bankLocation))
	assert.False(t, trading)
	assert.False(t, partial)
	trading, partial = c.IsTradingDay(time.Date(2023, 8, 6, 0, 0, 0, 0, c.bankLocation))
	assert.False(t, trading)
	assert.False(t, partial)
	trading, partial = c.IsTradingDay(time.Date(2023, 8, 7, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.False(t, partial)
	trading, partial = c.IsTradingDay(time.Date(2023, 8, 8, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.False(t, partial)
	trading, partial = c.IsTradingDay(time.Date(2023, 8, 9, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.False(t, partial)
	trading, partial = c.IsTradingDay(time.Date(2023, 8, 10, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.False(t, partial)
	trading, partial = c.IsTradingDay(time.Date(2023, 8, 11, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.False(t, partial)
	trading, partial = c.IsTradingDay(time.Date(2023, 8, 12, 0, 0, 0, 0, c.bankLocation))
	assert.False(t, trading)
	assert.False(t, partial)
	trading, partial = c.IsTradingDay(time.Date(2023, 8, 13, 0, 0, 0, 0, c.bankLocation))
	assert.False(t, trading)
	assert.False(t, partial)
}

func TestGetTradingHoursNormal(t *testing.T) {
	c := NewUSBankCalendar()
	trading, partial, h := c.GetTradingHours(time.Date(2023, 8, 9, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.False(t, partial)
	assert.True(t, h.Open.Equal(time.Date(2023, 8, 9, 9, 30, 0, 0, c.bankLocation)))
	assert.True(t, h.Close.Equal(time.Date(2023, 8, 9, 16, 0, 0, 0, c.bankLocation)))
	assert.True(t, h.PreOpen.Equal(time.Date(2023, 8, 9, 4, 0, 0, 0, c.bankLocation)))
	assert.True(t, h.ExtClose.Equal(time.Date(2023, 8, 9, 20, 0, 0, 0, c.bankLocation)))
}

func TestGetTradingHoursPartial(t *testing.T) {
	c := NewUSBankCalendar()
	// Christmas eve 2018 was a partial trading day.
	trading, partial, h := c.GetTradingHours(time.Date(2018, 12, 24, 0, 0, 0, 0, c.bankLocation))
	assert.True(t, trading)
	assert.True(t, partial)
	assert.True(t, h.Open.Equal(time.Date(2018, 12, 24, 9, 30, 0, 0, c.bankLocation)))
	assert.True(t, h.Close.Equal(time.Date(2018, 12, 24, 13, 0, 0, 0, c.bankLocation)))
	assert.True(t, h.PreOpen.Equal(time.Date(2018, 12, 24, 4, 0, 0, 0, c.bankLocation)))
	assert.True(t, h.ExtClose.Equal(time.Date(2018, 12, 24, 17, 0, 0, 0, c.bankLocation)))
}
