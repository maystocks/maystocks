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

type Capabilities struct {
	RealtimeBidAsk bool
	PaperTrading   bool
}

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
	GetCapabilities() Capabilities
	RemainingApiLimit() int
	ReadConfig(c config.Config) error
	FindAsset(ctx context.Context, entry <-chan SearchRequest, response chan<- SearchResponse)
}

type CandlesRequest struct {
	Asset      stockval.AssetData
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

type RealtimeDataSubscription int32

const (
	RealtimeTradesSubscribe RealtimeDataSubscription = iota
	RealtimeTradesUnsubscribe
	RealtimeBidAskSubscribe
	RealtimeBidAskUnsubscribe
)

type SubscribeDataRequest struct {
	Asset stockval.AssetData
	Type  RealtimeDataSubscription
}

type SubscribeDataResponse struct {
	Figi       string
	Error      error
	Type       RealtimeDataSubscription
	TickData   chan stockval.RealtimeTickData
	BidAskData chan stockval.RealtimeBidAskData
}

type OrderType int32

const (
	OrderTypeMarket OrderType = iota
	OrderTypeLimit
	OrderTypeStop
	OrderTypeStopLimit
	OrderTypeTrailingStop
)

type OrderTimeInForce int32

const (
	OrderTimeInForceDay OrderTimeInForce = iota
	OrderTimeInForceGtc
	OrderTimeInForceOpg
	OrderTimeInForceCls
	OrderTimeInForceIoc
	OrderTimeInForceFok
)

type TradeRequest struct {
	RequestId     string
	Asset         stockval.AssetData
	Quantity      *decimal.Big
	Sell          bool
	Type          OrderType
	LimitPrice    *decimal.Big
	TimeInForce   OrderTimeInForce
	ExtendedHours bool
}

type TradeResponse struct {
	RequestId string
	Figi      string
	OrderId   string
	Error     error
}

type Broker interface {
	SymbolSearchTool
	QueryQuote(ctx context.Context, entry <-chan stockval.AssetData, response chan<- QueryQuoteResponse)
	QueryCandles(ctx context.Context, request <-chan CandlesRequest, response chan<- QueryCandlesResponse)
	SubscribeData(ctx context.Context, request <-chan SubscribeDataRequest, response chan<- SubscribeDataResponse)
	TradeAsset(ctx context.Context, request <-chan TradeRequest, response chan<- TradeResponse, paperTrading bool)
}
