// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package calendar

import (
	"time"

	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/us"
)

const observedHolidayPostfix = "(observed)"

type BankCalendar struct {
	bankLocation            *time.Location
	calendar                *cal.BusinessCalendar
	stdOpenTime             bankTime
	stdCloseTime            bankTime
	partialCloseTime        bankTime
	extendedHoursBeforeOpen time.Duration
	extendedHoursAfterClose time.Duration
}

type bankTime struct {
	hours   int
	minutes int
}

func NewUSBankCalendar() BankCalendar {
	// NYSE uses ET, which can be either EST or EDT.
	// Luckily, changing to/from daylight saving time does not occur during market hours.
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic("NYSE time location not supported")
	}
	cal := cal.NewBusinessCalendar()
	// Source for bank holidays: https://www.federalreserve.gov/aboutthefed/k8.htm
	cal.AddHoliday(
		us.NewYear,
		us.MlkDay,
		us.PresidentsDay,
		us.MemorialDay,
		us.Juneteenth,
		us.IndependenceDay,
		us.LaborDay,
		us.ColumbusDay,
		us.VeteransDay,
		us.ThanksgivingDay,
		us.ChristmasDay,
	)
	cal.Cacheable = true
	return BankCalendar{
		calendar:                cal,
		bankLocation:            loc,
		stdOpenTime:             bankTime{hours: 9, minutes: 30},
		stdCloseTime:            bankTime{hours: 16, minutes: 0},
		partialCloseTime:        bankTime{hours: 13, minutes: 0},
		extendedHoursBeforeOpen: time.Hour*5 + time.Minute*30,
		extendedHoursAfterClose: time.Hour * 4,
	}
}

func (b BankCalendar) IsBankHoliday(t time.Time) (bool, string) {
	actual, observed, h := b.calendar.IsHoliday(t.In(b.bankLocation))
	if !actual && !observed {
		return false, ""
	} else if !actual {
		return true, h.Name + " " + observedHolidayPostfix
	} else {
		return true, h.Name
	}
}

func (b BankCalendar) IsTradingDay(t time.Time) (trading bool, partial bool) {
	day := t.In(b.bankLocation)
	trading = b.calendar.IsWorkday(day)

	if trading {
		holiday, name := b.IsBankHoliday(day.AddDate(0, 0, 1))
		// There are partial trading days before independence day and christmas.
		if holiday && (name == us.IndependenceDay.Name || name == us.ChristmasDay.Name) {
			partial = true
		} else {
			// There is a partial trading day before thanksgiving
			holiday, name = b.IsBankHoliday(day.AddDate(0, 0, -1))
			if holiday && name == us.ThanksgivingDay.Name {
				partial = true
			}
		}
	}
	return
}

func (b BankCalendar) GetTradingHours(t time.Time) (trading, partial bool, h TradingHours) {
	day := t.In(b.bankLocation)
	trading, partial = b.IsTradingDay(day)
	if !trading {
		return
	}
	y, m, d := day.Date()
	h.Open = time.Date(y, m, d, b.stdOpenTime.hours, b.stdOpenTime.minutes, 0, 0, b.bankLocation)
	if partial {
		h.Close = time.Date(y, m, d, b.partialCloseTime.hours, b.partialCloseTime.minutes, 0, 0, b.bankLocation)
	} else {
		h.Close = time.Date(y, m, d, b.stdCloseTime.hours, b.stdCloseTime.minutes, 0, 0, b.bankLocation)
	}
	h.PreOpen = h.Open.Add(-b.extendedHoursBeforeOpen)
	h.ExtClose = h.Close.Add(b.extendedHoursAfterClose)
	return
}
