// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockplot

import (
	"image"
	"math"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/stockval"
	"maystocks/widgets"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"github.com/stretchr/testify/assert"
)

func NewTestPlot() *Plot {
	theme := widgets.NewDarkPlotTheme()
	return NewPlot(
		theme,
		candles.CandleOneMinute,
		stockval.PlotScaling{},
		[]SubPlotData{
			{Type: indapi.SubPlotTypePrice},
			{Type: indapi.SubPlotTypeVolume},
		},
	)
}

func InitializeTestPlot(testPlot *Plot) {
	var ops op.Ops
	var gtx layout.Context
	gtx.Constraints.Max = image.Pt(800, 600)
	gtx.Ops = &ops
	testPlot.InitializeFrame(gtx, candles.CandleOneMinute)
}

func TestDefaultGridValueY(t *testing.T) {
	plot := NewTestPlot()
	InitializeTestPlot(plot)

	baseValue := plot.Sub[0].calcFirstGridValueY()

	assert.Equal(t, 0.0, baseValue)
}

func TestCalcFirstGridValueY(t *testing.T) {
	plot := NewTestPlot()
	plot.Sub[0].zeroValueY = 1.02
	InitializeTestPlot(plot)

	baseValue := plot.Sub[0].calcFirstGridValueY()

	assert.Equal(t, 1.1, baseValue)
}

func TestGetYpos(t *testing.T) {
	plot := NewTestPlot()
	plot.Sub[0].zeroValueY = 1.02
	InitializeTestPlot(plot)

	posY := plot.Sub[0].frame.projection.getYpos(1.2)

	assert.True(t, math.Abs(220.0-posY) < stockval.NearZero)
}
