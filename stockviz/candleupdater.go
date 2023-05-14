// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockviz

import (
	"context"
	"log"
	"maystocks/indapi/candles"
	"maystocks/stockapi"
	"maystocks/stockval"
	"time"

	"github.com/zhangyunhao116/skipmap"
)

type CandleUpdater struct {
	Entry               stockval.AssetData
	CandleData          *stockval.CandlePlotData
	candleTimeMap       *skipmap.Int32Map[candleTime]
	candlesRequestChan  chan stockapi.CandlesRequest
	candlesResponseChan chan stockapi.QueryCandlesResponse
}

type candleTime struct {
	firstCandleTime time.Time
	lastCandleTime  time.Time
}

func NewCandleUpdater(entry stockval.AssetData, resolution candles.CandleResolution) CandleUpdater {
	return CandleUpdater{
		Entry:         entry,
		CandleData:    stockval.NewCandlePlotData(resolution),
		candleTimeMap: skipmap.NewInt32[candleTime](),
	}
}

func (d *CandleUpdater) Initialize(ctx context.Context,
	stockValueRequester stockapi.StockValueRequester, uiUpdater StockUiUpdater) {
	// TODO size of buffered channels?
	d.candlesRequestChan = make(chan stockapi.CandlesRequest, 128)
	d.candlesResponseChan = make(chan stockapi.QueryCandlesResponse, 128)
	go func() {
		for candlesResponseData := range d.candlesResponseChan {
			log.Printf("Updating candle data %s %d.", candlesResponseData.Figi, candlesResponseData.Resolution)
			d.CandleData.UpdateConsolidatedCandles(candlesResponseData.Resolution, candlesResponseData.Data)
			uiUpdater.Invalidate()
		}
		log.Printf("Terminating candle update handler %s.", d.Entry.Figi)
	}()
	go stockValueRequester.QueryCandles(ctx, d.candlesRequestChan, d.candlesResponseChan)
}

func (d *CandleUpdater) Refresh() {
	d.candleTimeMap.Range(
		func(uiIndex int32, w candleTime) bool {
			log.Printf("Requesting candle data %s %d.", d.Entry.Figi, d.CandleData.Resolution)
			candlesRequestData := stockapi.CandlesRequest{
				Stock:      d.Entry,
				Resolution: d.CandleData.Resolution,
				FromTime:   w.firstCandleTime,
				ToTime:     w.lastCandleTime,
			}
			// TODO may send on closed chan?
			d.candlesRequestChan <- candlesRequestData
			return true
		},
	)
}

func (d *CandleUpdater) Cleanup() {
	close(d.candlesRequestChan)
}

func (d *CandleUpdater) SetCandleTime(uiIndex int32, first time.Time, last time.Time) {
	c := candleTime{firstCandleTime: first, lastCandleTime: last}
	d.candleTimeMap.Store(uiIndex, c)
}
