// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockviz

import (
	"context"
	"log"
	"maystocks/indapi/candles"
	"maystocks/stockapi"
	"maystocks/stockval"
	"sync"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/zhangyunhao116/skipmap"
)

type PriceData struct {
	Entry               stockval.AssetData
	RealtimeData        *skipmap.Int64Map[*decimal.Big]
	candles             map[candles.CandleResolution]CandleUpdater
	candlesMutex        *sync.Mutex
	quote               *stockval.QuoteData
	quoteMutex          *sync.Mutex
	stockValueRequester stockapi.StockValueRequester
	uiUpdater           StockUiUpdater
	quoteRequestChan    chan stockval.AssetData
	quoteResponseChan   chan stockapi.QueryQuoteResponse
}

func NewPriceData(entry stockval.AssetData) PriceData {
	return PriceData{
		Entry:        entry,
		RealtimeData: skipmap.NewInt64[*decimal.Big](),
		candles:      map[candles.CandleResolution]CandleUpdater{},
		candlesMutex: new(sync.Mutex),
		quote:        new(stockval.QuoteData),
		quoteMutex:   new(sync.Mutex),
	}
}

func (p *PriceData) Initialize(ctx context.Context, stockValueRequester stockapi.StockValueRequester, uiUpdater StockUiUpdater) {
	p.stockValueRequester = stockValueRequester
	p.uiUpdater = uiUpdater
	// TODO size of buffered channels?
	p.quoteRequestChan = make(chan stockval.AssetData, 128)
	p.quoteResponseChan = make(chan stockapi.QueryQuoteResponse, 128)
	go func() {
		for quoteResponseData := range p.quoteResponseChan {
			log.Printf("Updating quote data %s.", quoteResponseData.Figi)
			p.quoteMutex.Lock()
			p.quote.PreviousDelayedClosePrice = quoteResponseData.PreviousClosePrice
			if p.quote.PreviousClosePrice == nil {
				p.quote.PreviousClosePrice = quoteResponseData.PreviousClosePrice
			}
			p.quote.CurrentDelayedPrice = quoteResponseData.CurrentPrice
			if p.quote.Type == stockval.QuoteTypeDelayed { // only update if no realtime data is available
				p.quote.CurrentPrice = quoteResponseData.CurrentPrice
				p.quote.CurrentPriceTimestamp = time.Now()
				p.quote.DeltaPercentage = quoteResponseData.DeltaPercentage
			}
			p.quoteMutex.Unlock()
			uiUpdater.Invalidate()
		}
		log.Printf("Terminating quote update handler %s.", p.Entry.Figi)
	}()
	go stockValueRequester.QueryQuote(ctx, p.quoteRequestChan, p.quoteResponseChan)
}

func (p *PriceData) Cleanup() {
	p.candlesMutex.Lock()
	defer p.candlesMutex.Unlock()
	for _, c := range p.candles {
		c.Cleanup()
	}
}

func (p *PriceData) GetQuoteCopy() stockval.QuoteData {
	p.quoteMutex.Lock()
	defer p.quoteMutex.Unlock()
	return *p.quote
}

func (p *PriceData) RefreshQuote() {
	p.quoteRequestChan <- p.Entry
}

func (p *PriceData) RefreshCandles(r candles.CandleResolution) bool {
	p.candlesMutex.Lock()
	candle, ok := p.candles[r]
	p.candlesMutex.Unlock()
	if ok {
		candle.Refresh()
	} else {
		log.Printf("Could not find candle data for refresh: %s, %d", p.Entry.Figi, r)
	}
	return ok
}

func (p *PriceData) LoadOrAddCandleResolution(ctx context.Context, candleResolution candles.CandleResolution) (CandleUpdater, bool) {
	p.candlesMutex.Lock()
	defer p.candlesMutex.Unlock()
	c, ok := p.candles[candleResolution]
	if !ok {
		c = NewCandleUpdater(p.Entry, candleResolution)
		c.Initialize(ctx, p.stockValueRequester, p.uiUpdater)
		p.candles[candleResolution] = c
	}
	return c, ok
}

func (p *PriceData) SetRealtimeTradesChan(realtimeChan chan stockapi.RealtimeTickData, uiUpdater StockUiUpdater) {
	go func() {
		for data := range realtimeChan {
			p.AddRealtimePriceData(data.Timestamp, data.Price, data.Volume, data.TradeContext)
			uiUpdater.Invalidate()
		}
		log.Printf("Realtime trades channel %s was closed.", p.Entry.Figi)
	}()
}

func (p *PriceData) SetRealtimeBidAskChan(realtimeChan chan stockapi.RealtimeBidAskData, uiUpdater StockUiUpdater) {
	go func() {
		for _ = range realtimeChan {
			//log.Printf("bid: %v, ask %v", data.BidPrice, data.AskPrice)
			// TODO handle bid/ask data
			uiUpdater.Invalidate()
		}
		log.Printf("Realtime bid/ask channel %s was closed.", p.Entry.Figi)
	}()
}

func (p *PriceData) AddRealtimePriceData(timestamp time.Time, price *decimal.Big, volume *decimal.Big,
	tradeContext stockval.TradeContext) {
	// IMPORTANT NOTE:
	// Do not assume that the order of realtime data is correct (ordered by time), sometimes it is not.

	// TODO add option to indicate all trades, maybe using animated dots.

	// Update last price only if price data indicates update.
	// We also implicitly update pre/post-market prices.
	if tradeContext.UpdateLast {
		p.RealtimeData.Store(timestamp.UnixMilli(), price)
		p.quoteMutex.Lock()
		if tradeContext.ExtendedHours {
			// use last delayed price as base for trade price difference outside of normal trading hours
			p.quote.PreviousClosePrice = p.quote.CurrentDelayedPrice
		}

		if p.quote.Type == stockval.QuoteTypeDelayed {
			p.quote.Type = stockval.QuoteTypeRealtime
			p.quote.CurrentPriceTimestamp = time.Time{}
		}
		if timestamp.After(p.quote.CurrentPriceTimestamp) {
			p.quote.CurrentPrice = price
			p.quote.CurrentPriceTimestamp = timestamp
			if p.quote.PreviousClosePrice != nil {
				p.quote.DeltaPercentage = stockval.CalculateDeltaPercentage(p.quote.PreviousClosePrice, p.quote.CurrentPrice)
			}
		}
		p.quoteMutex.Unlock()
	}

	p.candlesMutex.Lock()
	for _, c := range p.candles {
		c.CandleData.AddRealtimeData(timestamp, price, volume, tradeContext)
	}
	p.candlesMutex.Unlock()
}
