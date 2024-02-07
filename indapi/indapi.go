// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package indapi

import (
	"image/color"
	"maystocks/indapi/candles"
	"sync"
	"time"

	"gioui.org/layout"
	"github.com/ericlagergren/decimal"
)

type IndicatorId string

// For sorting
type IndicatorList []IndicatorId

func (x IndicatorList) Len() int           { return len(x) }
func (x IndicatorList) Less(i, j int) bool { return x[i] < x[j] }
func (x IndicatorList) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type CandleData struct {
	Timestamp  time.Time
	OpenPrice  *decimal.Big
	HighPrice  *decimal.Big
	LowPrice   *decimal.Big
	ClosePrice *decimal.Big
	Volume     *decimal.Big
}

// For sorting
type CandleList []CandleData

func (x CandleList) Len() int           { return len(x) }
func (x CandleList) Less(i, j int) bool { return x[i].Timestamp.Before(x[j].Timestamp) }
func (x CandleList) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type PlotData struct {
	Data           []CandleData
	DataLastChange time.Time
	DataMutex      *sync.RWMutex
	Cache          struct {
		LastUpdate  time.Time
		Timestamps  []time.Time
		OpenPrices  []float64
		HighPrices  []float64
		LowPrices   []float64
		ClosePrices []float64
		Volumes     []float64
	}
}

type SubPlotType int

const (
	SubPlotTypePrice SubPlotType = iota
	SubPlotTypeVolume
	SubPlotTypeIndicator
)

type LinePlotter interface {
	PlotLine(timestamps []time.Time, data []float64, maxValue *float64, r candles.CandleResolution, c color.NRGBA, gtx layout.Context)
}

type IndicatorData interface {
	Update(r candles.CandleResolution, data *PlotData)
	Plot(p LinePlotter, maxValue *float64, defaultColor color.NRGBA, gtx layout.Context)
	GetId() IndicatorId
	GetProperties() map[string]string
	SetProperties(map[string]string)
	GetColors() []color.NRGBA
	SetColors([]color.NRGBA)
	GetSubPlotType() SubPlotType
}

func GetMinColors(c []color.NRGBA, numColors int) []color.NRGBA {
	for len(c) < numColors {
		c = append(c, color.NRGBA{})
	}
	return c
}

func GetNormalisedColors(c []color.NRGBA, def color.NRGBA) []color.NRGBA {
	for i := range c {
		if empty := (color.NRGBA{}); c[i] == empty {
			c[i] = def
		}
	}
	return c
}
