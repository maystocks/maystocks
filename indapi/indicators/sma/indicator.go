// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package sma

import (
	"image/color"
	"maystocks/indapi"
	"maystocks/indapi/calc"
	"maystocks/indapi/candles"
	"maystocks/indapi/properties"
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/widget/material"
	"github.com/ericlagergren/decimal"
)

type Indicator struct {
	resolution     candles.CandleResolution
	timestamps     []time.Time
	sma            []*decimal.Big
	dataLastChange time.Time
	numPeriods     int
	color          color.NRGBA
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
		d.timestamps = d.timestamps[:0]
		d.sma = d.sma[:0]
		for i := range data.Data {
			d.timestamps = append(d.timestamps, data.Data[i].Timestamp)
			subSet := data.Data[calc.Max(0, i+1-d.numPeriods) : i+1]
			if len(subSet) > 0 {
				mean := calc.Mean(new(decimal.Big), subSet)
				d.sma = append(d.sma, mean)
			}
		}
	}
}

func (d *Indicator) Plot(p indapi.LinePlotter, gtx layout.Context, th *material.Theme) {
	c := d.color
	if empty := (color.NRGBA{}); c == empty {
		c = th.Fg
	}
	p.PlotLine(d.timestamps, d.sma, d.resolution, c, gtx)
}
