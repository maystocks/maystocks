// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package bollinger

import (
	"image/color"
	"log"
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
	top            []float64
	mid            []float64
	bottom         []float64
	dataLastChange time.Time
	timeUnits      int
	colors         []color.NRGBA
}

const Id = "bollinger"

func NewIndicator() indapi.IndicatorData {
	return &Indicator{timeUnits: 20}
}

func (d *Indicator) GetId() indapi.IndicatorId {
	return Id
}

func (d *Indicator) GetProperties() map[string]string {
	return map[string]string{
		"Time Units": strconv.Itoa(d.timeUnits),
	}
}

func (d *Indicator) SetProperties(prop map[string]string) {
	for key, value := range prop {
		switch key {
		case "Time Units":
			properties.SetPositiveValue(&d.timeUnits, value)
		default:
			log.Printf("Unknown property %s was ignored.", key)
		}
	}
}

func (d *Indicator) GetColors() []color.NRGBA {
	return indapi.GetMinColors(d.colors, 3)
}

func (d *Indicator) SetColors(c []color.NRGBA) {
	d.colors = c
}

func (d *Indicator) Update(r candles.CandleResolution, data *indapi.PlotData) {
	data.DataMutex.Lock()
	defer data.DataMutex.Unlock()
	if !d.dataLastChange.Equal(data.DataLastChange) { // TODO this should be generic for all indicators
		d.dataLastChange = data.DataLastChange
		d.resolution = r

		d.mid, d.top, d.bottom = indicator.BollingerBands(data.Cache.ClosePrices)
		d.timestamps = data.Cache.Timestamps
	}
}

func (d *Indicator) Plot(p indapi.LinePlotter, maxValue *float64, defaultColor color.NRGBA, gtx layout.Context) {
	c := indapi.GetNormalisedColors(d.GetColors(), defaultColor)
	p.PlotLine(d.timestamps[0:len(d.top)], d.top, maxValue, d.resolution, c[0], gtx)
	p.PlotLine(d.timestamps[0:len(d.mid)], d.mid, maxValue, d.resolution, c[1], gtx)
	p.PlotLine(d.timestamps[0:len(d.bottom)], d.bottom, maxValue, d.resolution, c[2], gtx)
}

func (d *Indicator) GetSubPlotType() indapi.SubPlotType {
	return indapi.SubPlotTypePrice
}
