// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockapi

import (
	"context"
	"maystocks/config"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/stockval"
	"time"

	"github.com/ericlagergren/decimal"
)

type SearchRequest struct {
	RequestId         string
	Text              string
	MaxNumResults     int
	UnambiguousLookup bool
}

type SearchResponse struct {
	SearchRequest
	Error  error
	Result []stockval.AssetData
}

type SymbolSearchTool interface {
	RemainingApiLimit() int
	ReadConfig(c config.Config) error
	FindAsset(ctx context.Context, entry <-chan SearchRequest, response chan<- SearchResponse)
}

type CandlesRequest struct {
	Stock      stockval.AssetData
	Resolution candles.CandleResolution
	FromTime   time.Time
	ToTime     time.Time
}

type QueryQuoteResponse struct {
	Figi               string
	Error              error
	CurrentPrice       *decimal.Big
	PreviousClosePrice *decimal.Big
	DeltaPercentage    *decimal.Big
}

type QueryCandlesResponse struct {
	Figi       string
	Resolution candles.CandleResolution
	Error      error
	Data       []indapi.CandleData
}

type RealtimeTickData struct {
	Timestamp    time.Time
	Price        *decimal.Big
	Volume       *decimal.Big
	TradeContext stockval.TradeContext
}

type SubscribeTradesRequest struct {
	Stock stockval.AssetData
	Type  stockval.RealtimeDataSubscription
}

type SubscribeTradesResponse struct {
	Figi     string
	Error    error
	Type     stockval.RealtimeDataSubscription
	TickData chan RealtimeTickData
}

type StockValueRequester interface {
	SymbolSearchTool
	QueryQuote(ctx context.Context, entry <-chan stockval.AssetData, response chan<- QueryQuoteResponse)
	QueryCandles(ctx context.Context, request <-chan CandlesRequest, response chan<- QueryCandlesResponse)
	SubscribeTrades(ctx context.Context, entry <-chan SubscribeTradesRequest, response chan<- SubscribeTradesResponse)
}
