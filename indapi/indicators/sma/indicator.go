// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package sma

import (
	"image/color"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/indapi/properties"
	"strconv"
	"time"

	"gioui.org/layout"
	"github.com/cinar/indicator"
)

type Indicator struct {
	resolution     candles.CandleResolution
	timestamps     []time.Time
	result         []float64
	dataLastChange time.Time
	numPeriods     int
	colors         []color.NRGBA
}

const Id = "sma"

func NewIndicator() indapi.IndicatorData {
	return &Indicator{numPeriods: 9}
}

func (d *Indicator) GetId() indapi.IndicatorId {
	return Id
}

func (d *Indicator) GetProperties() map[string]string {
	return map[string]string{
		"Time Periods": strconv.Itoa(d.numPeriods),
	}
}

func (d *Indicator) SetProperties(prop map[string]string) {
	for key, value := range prop {
		switch key {
		case "Time Periods":
			properties.SetPositiveValue(&d.numPeriods, value)
		default:
			panic("unknown indicator property")
		}
	}
}

func (d *Indicator) GetColors() []color.NRGBA {
	return d.colors
}

func (d *Indicator) SetColors(c []color.NRGBA) {
	d.colors = indapi.GetMinColors(c, 1)
}

func (d *Indicator) Update(r candles.CandleResolution, data *indapi.PlotData) {
	data.DataMutex.Lock()
	defer data.DataMutex.Unlock()
	if !d.dataLastChange.Equal(data.DataLastChange) { // TODO this should be generic for all indicators
		d.dataLastChange = data.DataLastChange
		d.resolution = r
		d.timestamps = data.Cache.Timestamps
		d.result = indicator.Sma(d.numPeriods, data.Cache.ClosePrices)
	}
}

func (d *Indicator) Plot(p indapi.LinePlotter, maxValue *float64, defaultColor color.NRGBA, gtx layout.Context) {
	c := indapi.GetNormalisedColors(d.colors, defaultColor)
	p.PlotLine(d.timestamps[0:len(d.result)], d.result, maxValue, d.resolution, c[0], gtx)
}

func (d *Indicator) GetSubPlotType() indapi.SubPlotType {
	return indapi.SubPlotTypePrice
}
