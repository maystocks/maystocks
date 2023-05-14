// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockval

import (
	"log"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"sort"
	"sync"
	"time"

	"github.com/ericlagergren/decimal"
)

const PreloadCandlesBefore = 64
const PreloadCandlesAfter = 64

type RealtimeData struct {
	// These are arrays, because some old data may still be received (with lag), and
	// stockapi may return consolidated candles with delay.
	indapi.PlotData
	OpenTimestamps   []time.Time
	CloseTimestamps  []time.Time
	OpenConsolidated []bool
	HasInitData      bool
}

type CandlePlotData struct {
	// The resolution is set during initialization and never changed.
	// Therefore it is safe to be accessed from different goroutines.
	Resolution candles.CandleResolution
	// The data contains "consolidated" candles only as returned by stockapi.
	// This candle data should not be updated by realtime data.
	// Timestamp is start of candle.
	indapi.PlotData
	// Realtime candle data can be updated. Candles will be appended to "consolidated" candles.
	RealtimeData RealtimeData
}

func NewCandlePlotData(resolution candles.CandleResolution) *CandlePlotData {
	return &CandlePlotData{
		Resolution: resolution,
		PlotData: indapi.PlotData{
			DataMutex: new(sync.RWMutex),
		},
		RealtimeData: RealtimeData{
			PlotData: indapi.PlotData{
				DataMutex: new(sync.RWMutex),
			},
		},
	}
}

func (d *CandlePlotData) GetLastConsolidatedTimestamp() (t time.Time, r candles.CandleResolution, ok bool) {
	d.DataMutex.RLock()
	defer d.DataMutex.RUnlock()
	r = d.Resolution
	dataSize := len(d.Data)
	if dataSize > 0 {
		t = d.Data[dataSize-1].Timestamp
		ok = true
	} else {
		ok = false
	}
	return
}

func (d *CandlePlotData) UpdateConsolidatedCandles(candleResolution candles.CandleResolution, data []indapi.CandleData) {
	// Treat last entry as dynamic data, as it may still be updated by realtime data.
	// TODO this may depend on stockapi broker. It was originally implemented for finnhub.
	// E.g. maybe use all data with alpaca.
	if len(data) == 0 {
		return // no data available
	}
	usableSize := len(data) - 1

	d.DataMutex.Lock()
	// Do not delete data, merge old data with new data
	d.Data = append(d.Data, data[:usableSize]...)
	sort.Stable(indapi.CandleList(d.Data))
	// Remove adjacent duplicates.
	k := 0
	for i := range d.Data {
		d.Data[k] = d.Data[i]
		if i < len(d.Data)-1 {
			if !d.Data[i].Timestamp.Equal(d.Data[i+1].Timestamp) {
				k++
			}
		} else {
			k++
		}
	}
	d.Data = d.Data[:k]
	d.DataLastChange = time.Now()
	d.DataMutex.Unlock()

	d.consolidateRealtimeData(candleResolution, data[usableSize])
}

func (d *CandlePlotData) AddRealtimeData(timestamp time.Time, price *decimal.Big, volume *decimal.Big, tradeContext TradeContext) {
	lastConsolidatedCandleTime, candleResolution, ok := d.GetLastConsolidatedTimestamp()
	// Update only if candle data has already been received.
	if !ok {
		return // wait for candle data before updating
	}
	candleIndex := candleResolution.GetDeltaCandleCount(lastConsolidatedCandleTime, timestamp)
	if candleIndex < 0 {
		log.Println("old candle data received, not updating")
		return
	}

	candleTime := candleResolution.GetNthCandleTime(lastConsolidatedCandleTime, candleIndex)

	// Either update existing realtime candle or add a new
	updated := false
	d.RealtimeData.DataMutex.Lock()
	defer d.RealtimeData.DataMutex.Unlock()
	for i := range d.RealtimeData.Data {
		if candleTime.Equal(d.RealtimeData.Data[i].Timestamp) {
			// Prices should never be modified. Therefore, we use the original price object without copying.
			if tradeContext.UpdateHighLow {
				if d.RealtimeData.Data[i].HighPrice == nil || price.Cmp(d.RealtimeData.Data[i].HighPrice) > 0 {
					d.RealtimeData.Data[i].HighPrice = price
				}
				if d.RealtimeData.Data[i].LowPrice == nil || price.Cmp(d.RealtimeData.Data[i].LowPrice) < 0 {
					d.RealtimeData.Data[i].LowPrice = price
				}
			}
			if tradeContext.UpdateLast {
				if d.RealtimeData.Data[i].OpenPrice == nil || d.RealtimeData.OpenTimestamps[i].IsZero() || timestamp.Before(d.RealtimeData.OpenTimestamps[i]) {
					d.RealtimeData.OpenTimestamps[i] = timestamp
					d.RealtimeData.Data[i].OpenPrice = price
				}
				if d.RealtimeData.Data[i].ClosePrice == nil || d.RealtimeData.CloseTimestamps[i].IsZero() || timestamp.After(d.RealtimeData.CloseTimestamps[i]) {
					d.RealtimeData.CloseTimestamps[i] = timestamp
					d.RealtimeData.Data[i].ClosePrice = price
				}
			}
			if tradeContext.UpdateVolume {
				d.RealtimeData.Data[i].Volume.Add(d.RealtimeData.Data[i].Volume, volume)
			}
			if d.RealtimeData.OpenConsolidated[i] && !d.RealtimeData.HasInitData {
				// There may still be some high/low difference, but open price is fine.
				// We consider this as initialized.
				d.RealtimeData.HasInitData = true
			}
			updated = true
			break
		}
	}
	if !updated {
		// Add new entries
		d.RealtimeData.Data = append(d.RealtimeData.Data, indapi.CandleData{
			Timestamp:  candleTime,
			OpenPrice:  price,
			HighPrice:  price,
			LowPrice:   price,
			ClosePrice: price,
			Volume:     new(decimal.Big).Copy(volume), // will be modified
		})
		d.RealtimeData.OpenTimestamps = append(d.RealtimeData.OpenTimestamps, timestamp)
		d.RealtimeData.CloseTimestamps = append(d.RealtimeData.CloseTimestamps, timestamp)
		// First candle has most probably incorrect open price, we need more realtime data.
		d.RealtimeData.OpenConsolidated = append(d.RealtimeData.OpenConsolidated, d.RealtimeData.HasInitData)
		if !d.RealtimeData.HasInitData {
			d.RealtimeData.HasInitData = true // next candle should be fine
		}
	}
	d.RealtimeData.DataLastChange = time.Now()
}

func (d *CandlePlotData) consolidateRealtimeData(candleResolution candles.CandleResolution, lastCandleData indapi.CandleData) {
	realtimeCandleExists := false
	//realtimeCandleIndex := -1
	// There may have been a banking holiday on the usual start of the candle,
	// so use normalised start time when comparing with realtime data.
	lastCandleUnixTime := candleResolution.GetNthCandleTime(lastCandleData.Timestamp, 0).Unix()
	lastRealtimeUnixTime := lastCandleUnixTime
	k := 0
	d.RealtimeData.DataMutex.Lock()
	defer d.RealtimeData.DataMutex.Unlock()
	d.DataMutex.RLock()
	defer d.DataMutex.RUnlock()
	for i := range d.RealtimeData.Data {
		consolidatedCandleExists := false
		consolidatedCandleIndex := -1
		realtimeUnixTime := d.RealtimeData.Data[i].Timestamp.Unix()
		if realtimeUnixTime == lastCandleUnixTime {
			realtimeCandleExists = true
			//realtimeCandleIndex = k
		} else {
			for ci, ct := range d.Data {
				if realtimeUnixTime == ct.Timestamp.Unix() {
					consolidatedCandleExists = true
					consolidatedCandleIndex = ci
				}
			}
			if realtimeUnixTime > lastRealtimeUnixTime {
				lastRealtimeUnixTime = realtimeUnixTime
			}
		}
		if consolidatedCandleExists {
			log.Println("replacing realtime candle")
			if d.HasValidRealtimePrices(i) &&
				(d.Data[consolidatedCandleIndex].OpenPrice.Cmp(d.RealtimeData.Data[i].OpenPrice) != 0 ||
					d.Data[consolidatedCandleIndex].HighPrice.Cmp(d.RealtimeData.Data[i].HighPrice) != 0 ||
					d.Data[consolidatedCandleIndex].LowPrice.Cmp(d.RealtimeData.Data[i].LowPrice) != 0 ||
					d.Data[consolidatedCandleIndex].ClosePrice.Cmp(d.RealtimeData.Data[i].ClosePrice) != 0 ||
					d.Data[consolidatedCandleIndex].Volume.Cmp(d.RealtimeData.Data[i].Volume) != 0) {
				log.Printf("inconsistent data t %v o %f:%f h %f:%f l %f:%f c %f:%f v %f:%f",
					d.Data[consolidatedCandleIndex].Timestamp,
					d.Data[consolidatedCandleIndex].OpenPrice, d.RealtimeData.Data[i].OpenPrice,
					d.Data[consolidatedCandleIndex].HighPrice, d.RealtimeData.Data[i].HighPrice,
					d.Data[consolidatedCandleIndex].LowPrice, d.RealtimeData.Data[i].LowPrice,
					d.Data[consolidatedCandleIndex].ClosePrice, d.RealtimeData.Data[i].ClosePrice,
					d.Data[consolidatedCandleIndex].Volume, d.RealtimeData.Data[i].Volume,
				)
			}
		} else {
			// keep non-consolidated entries
			d.RealtimeData.Data[k] = d.RealtimeData.Data[i]
			d.RealtimeData.OpenTimestamps[k] = d.RealtimeData.OpenTimestamps[i]
			d.RealtimeData.CloseTimestamps[k] = d.RealtimeData.CloseTimestamps[i]
			d.RealtimeData.OpenConsolidated[k] = d.RealtimeData.OpenConsolidated[i]
			k++
		}
	}
	d.RealtimeData.Data = d.RealtimeData.Data[:k]
	d.RealtimeData.OpenTimestamps = d.RealtimeData.OpenTimestamps[:k]
	d.RealtimeData.CloseTimestamps = d.RealtimeData.CloseTimestamps[:k]
	d.RealtimeData.OpenConsolidated = d.RealtimeData.OpenConsolidated[:k]
	/*if realtimeCandleExists {
		// If this is not the last candle, it will probably no longer be updated by realtime data.
		// This means we should update the data.
		if d.RealtimeData.Data[realtimeCandleIndex].Timestamp.Unix() < lastRealtimeUnixTime {
			d.RealtimeData.Data[realtimeCandleIndex].OpenPrice = lastCandleData.OpenPrice
			d.RealtimeData.Data[realtimeCandleIndex].HighPrice = lastCandleData.HighPrice
			d.RealtimeData.Data[realtimeCandleIndex].LowPrice = lastCandleData.LowPrice
			d.RealtimeData.Data[realtimeCandleIndex].ClosePrice = lastCandleData.ClosePrice
			d.RealtimeData.Data[realtimeCandleIndex].Volume = new(decimal.Big).Copy(lastCandleData.Volume)
			// Assume that data is now complete (even though it may still need some correction afterwards).
			if !d.RealtimeData.OpenConsolidated[realtimeCandleIndex] {
				d.RealtimeData.OpenConsolidated[realtimeCandleIndex] = true
			}
		}
	}*/
	if !realtimeCandleExists {
		// prepend the latest non-consolidated candle to initialize realtime data
		prependCandle := lastCandleData
		// Copy volume data, because it will be modified.
		prependCandle.Volume = new(decimal.Big).Copy(prependCandle.Volume)
		d.RealtimeData.Data = append([]indapi.CandleData{prependCandle}, d.RealtimeData.Data...)
		d.RealtimeData.OpenTimestamps = append([]time.Time{lastCandleData.Timestamp}, d.RealtimeData.OpenTimestamps...)
		d.RealtimeData.CloseTimestamps = append([]time.Time{{}}, d.RealtimeData.CloseTimestamps...)
		// Partial data. High/low may be missing, but we assume it is OK.
		d.RealtimeData.OpenConsolidated = append([]bool{true}, d.RealtimeData.OpenConsolidated...)
	}
}

func (d *CandlePlotData) HasValidRealtimePrices(i int) bool {
	return d.RealtimeData.Data[i].OpenPrice != nil && d.RealtimeData.Data[i].HighPrice != nil && d.RealtimeData.Data[i].LowPrice != nil && d.RealtimeData.Data[i].ClosePrice != nil
}
