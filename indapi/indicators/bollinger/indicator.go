// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package bollinger

import (
	"image/color"
	"log"
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
	top            []*decimal.Big
	mid            []*decimal.Big
	bottom         []*decimal.Big
	dataLastChange time.Time
	timeUnits      int
	bandWidth      int
	color          color.NRGBA
}

const Id = "bollinger"

func NewIndicator() indapi.IndicatorData {
	return &Indicator{timeUnits: 20, bandWidth: 2}
}

func (d *Indicator) GetId() indapi.IndicatorId {
	return Id
}

func (d *Indicator) GetProperties() map[string]string {
	return map[string]string{
		"Width":      strconv.Itoa(d.bandWidth),
		"Time Units": strconv.Itoa(d.timeUnits),
	}
}

func (d *Indicator) SetProperties(prop map[string]string) {
	for key, value := range prop {
		switch key {
		case "Width":
			properties.SetPositiveValue(&d.bandWidth, value)
		case "Time Units":
			properties.SetPositiveValue(&d.timeUnits, value)
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
		d.timestamps = d.timestamps[:0]
		d.top = d.top[:0]
		d.mid = d.mid[:0]
		d.bottom = d.bottom[:0]
		for i := range data.Data {
			d.timestamps = append(d.timestamps, data.Data[i].Timestamp)
			subSet := data.Data[max(0, i+1-d.timeUnits) : i+1]
			if len(subSet) > 0 {
				mean := calc.Mean(new(decimal.Big), subSet)
				meanTop := new(decimal.Big).Copy(mean)
				meanBottom := new(decimal.Big).Copy(mean)
				stdDev := calc.StdDev(new(decimal.Big), subSet)
				stdDev.Mul(stdDev, decimal.New(int64(d.bandWidth), 0))
				d.top = append(d.top, meanTop.Sub(meanTop, stdDev))
				d.mid = append(d.mid, mean)
				d.bottom = append(d.bottom, meanBottom.Add(meanBottom, stdDev))
			}
		}
	}
}

func (d *Indicator) Plot(p indapi.LinePlotter, gtx layout.Context, th *material.Theme) {
	c := d.color
	if empty := (color.NRGBA{}); c == empty {
		c = th.Fg
	}
	p.PlotLine(d.timestamps, d.top, d.resolution, c, gtx)
	p.PlotLine(d.timestamps, d.mid, d.resolution, c, gtx)
	p.PlotLine(d.timestamps, d.bottom, d.resolution, c, gtx)
}
