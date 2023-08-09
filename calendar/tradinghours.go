// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package calendar

import "time"

type TradingHours struct {
	Open     time.Time
	Close    time.Time
	PreOpen  time.Time
	ExtClose time.Time
}

func (h TradingHours) GetTradingState(t time.Time) string {
	if t.Before(h.PreOpen) || t.After(h.ExtClose) {
		return "Market Closed"
	} else if t.Before(h.Open) {
		return "Pre-Market"
	} else if t.Before(h.Close) {
		return ""
	} else {
		return "After-Hours"
	}
}
