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
	Entry             stockval.AssetData
	RealtimeData      *skipmap.Int64Map[*decimal.Big]
	candles           map[candles.CandleResolution]CandleUpdater
	candlesMutex      *sync.Mutex
	quote             *stockval.QuoteData
	quoteMutex        *sync.Mutex
	bidAsk            *stockval.RealtimeBidAskData
	bidAskMutex       *sync.Mutex
	broker            stockapi.Broker
	uiUpdater         StockUiUpdater
	quoteRequestChan  chan stockval.AssetData
	quoteResponseChan chan stockapi.QueryQuoteResponse
}

func NewPriceData(entry stockval.AssetData) PriceData {
	return PriceData{
		Entry:        entry,
		RealtimeData: skipmap.NewInt64[*decimal.Big](),
		candles:      map[candles.CandleResolution]CandleUpdater{},
		candlesMutex: new(sync.Mutex),
		quote:        new(stockval.QuoteData),
		quoteMutex:   new(sync.Mutex),
		bidAsk:       new(stockval.RealtimeBidAskData),
		bidAskMutex:  new(sync.Mutex),
	}
}

func (p *PriceData) Initialize(ctx context.Context, broker stockapi.Broker, uiUpdater StockUiUpdater) {
	p.broker = broker
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
	go broker.QueryQuote(ctx, p.quoteRequestChan, p.quoteResponseChan)
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

func (p *PriceData) GetBidAskCopy() stockval.RealtimeBidAskData {
	p.bidAskMutex.Lock()
	defer p.bidAskMutex.Unlock()
	return *p.bidAsk
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
		c.Initialize(ctx, p.broker, p.uiUpdater)
		p.candles[candleResolution] = c
	}
	return c, ok
}

func (p *PriceData) SetRealtimeTradesChan(realtimeChan chan stockval.RealtimeTickData, uiUpdater StockUiUpdater) {
	go func() {
		for data := range realtimeChan {
			p.AddRealtimePriceData(data)
			uiUpdater.Invalidate()
		}
		log.Printf("Realtime trades channel %s was closed.", p.Entry.Figi)
	}()
}

func (p *PriceData) SetRealtimeBidAskChan(realtimeChan chan stockval.RealtimeBidAskData, uiUpdater StockUiUpdater) {
	go func() {
		for data := range realtimeChan {
			p.AddRealtimeBidAskData(data)
			uiUpdater.Invalidate()
		}
		log.Printf("Realtime bid/ask channel %s was closed.", p.Entry.Figi)
	}()
}

func (p *PriceData) AddRealtimePriceData(data stockval.RealtimeTickData) {
	// IMPORTANT NOTE:
	// Do not assume that the order of realtime data is correct (ordered by time), sometimes it is not.

	// TODO add option to indicate all trades, maybe using animated dots.

	// Update last price only if price data indicates update.
	// We also implicitly update pre/post-market prices.
	if data.TradeContext.UpdateLast {
		p.RealtimeData.Store(data.Timestamp.UnixMilli(), data.Price)
		p.quoteMutex.Lock()
		if data.TradeContext.ExtendedHours {
			// use last delayed price as base for trade price difference outside of normal trading hours
			p.quote.PreviousClosePrice = p.quote.CurrentDelayedPrice
		}

		if p.quote.Type == stockval.QuoteTypeDelayed {
			p.quote.Type = stockval.QuoteTypeRealtime
			p.quote.CurrentPriceTimestamp = time.Time{}
		}
		if data.Timestamp.After(p.quote.CurrentPriceTimestamp) {
			p.quote.CurrentPrice = data.Price
			p.quote.CurrentPriceTimestamp = data.Timestamp
			if p.quote.PreviousClosePrice != nil {
				p.quote.DeltaPercentage = stockval.CalculateDeltaPercentage(p.quote.PreviousClosePrice, p.quote.CurrentPrice)
			}
		}
		p.quoteMutex.Unlock()
	}

	p.candlesMutex.Lock()
	for _, c := range p.candles {
		c.CandleData.AddRealtimeData(data.Timestamp, data.Price, data.Volume, data.TradeContext)
	}
	p.candlesMutex.Unlock()
}

func (p *PriceData) AddRealtimeBidAskData(data stockval.RealtimeBidAskData) {
	p.bidAskMutex.Lock()
	*p.bidAsk = data // Simply replace the previous values.
	p.bidAskMutex.Unlock()
}
