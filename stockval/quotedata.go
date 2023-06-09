// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockval

import (
	"time"

	"github.com/ericlagergren/decimal"
)

type QuoteType int

const (
	QuoteTypeDelayed QuoteType = iota
	QuoteTypeRealtime
)

type QuoteData struct {
	CurrentRealtimePrice      *decimal.Big
	CurrentDelayedPrice       *decimal.Big
	CurrentPrice              *decimal.Big
	CurrentPriceTimestamp     time.Time
	Type                      QuoteType
	PreviousDelayedClosePrice *decimal.Big
	PreviousClosePrice        *decimal.Big
	DeltaPercentage           *decimal.Big
}

type RealtimeTickData struct {
	Timestamp    time.Time
	Price        *decimal.Big
	Volume       *decimal.Big
	TradeContext TradeContext
}

type RealtimeBidAskData struct {
	Timestamp time.Time
	BidPrice  *decimal.Big
	BidSize   uint
	AskPrice  *decimal.Big
	AskSize   uint
}
