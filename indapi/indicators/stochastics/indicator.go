// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stochastics

import (
	"image/color"
	"log"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"time"

	"gioui.org/layout"
	"gioui.org/widget/material"
	"github.com/cinar/indicator"
)

type Indicator struct {
	resolution     candles.CandleResolution
	timestamps     []time.Time
	k              []float64
	d              []float64
	dataLastChange time.Time
	color          color.NRGBA
}

const Id = "stochastics"

func NewIndicator() indapi.IndicatorData {
	return &Indicator{}
}

func (d *Indicator) GetId() indapi.IndicatorId {
	return Id
}

func (d *Indicator) GetProperties() map[string]string {
	return map[string]string{}
}

func (d *Indicator) SetProperties(prop map[string]string) {
	for key := range prop {
		switch key {
		default:
			log.Printf("Unknown property %s was ignored.", key)
		}
	}
}

func (d *Indicator) GetColor() color.NRGBA {
	return d.color
}

func (d *Indicator) SetColor(c color.NRGBA) {
	d.color = c
}

func (d *Indicator) Update(r candles.CandleResolution, data *indapi.PlotData) {
	data.DataMutex.Lock()
	defer data.DataMutex.Unlock()
	if !d.dataLastChange.Equal(data.DataLastChange) { // TODO this should be generic for all indicators
		d.dataLastChange = data.DataLastChange
		d.resolution = r

		d.k, d.d = indicator.StochasticOscillator(data.Cache.HighPrices, data.Cache.LowPrices, data.Cache.ClosePrices)
		d.timestamps = data.Cache.Timestamps
	}
}

func (d *Indicator) Plot(p indapi.LinePlotter, maxValue *float64, gtx layout.Context, th *material.Theme) {
	c := d.color
	if empty := (color.NRGBA{}); c == empty {
		c = th.Fg
	}
	p.PlotLine(d.timestamps[0:len(d.k)], d.k, maxValue, d.resolution, c, gtx)
	p.PlotLine(d.timestamps[0:len(d.d)], d.d, maxValue, d.resolution, c, gtx)
}

func (d *Indicator) GetSubPlotType() indapi.SubPlotType {
	return indapi.SubPlotTypeIndicator
}
