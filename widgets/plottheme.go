// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
)

type DpPoint struct {
	X unit.Dp
	Y unit.Dp
}

func (p *DpPoint) Dp(gtx layout.Context) image.Point {
	return image.Point{
		X: gtx.Dp(p.X),
		Y: gtx.Dp(p.Y),
	}
}

type PlotTheme struct {
	AxesMarginMin                DpPoint
	AxesMarginMax                DpPoint
	SubPlotMarginY               unit.Dp
	TextMargin                   DpPoint
	AxesXfontSize                int
	AxesYfontSize                int
	DefaultPlotGrid              DpPoint
	DefaultTimeUnitGrid          float64
	AxesColor                    color.NRGBA
	GridColor                    color.NRGBA
	CandleUnknownColor           color.NRGBA
	CandleUpColor                color.NRGBA
	CandleDownColor              color.NRGBA
	CandleUpBorderColor          color.NRGBA
	CandleDownBorderColor        color.NRGBA
	UseBorderColorForCandleLines bool
	DrawCandleUpBorder           bool
	DrawCandleDownBorder         bool
	BarUnknownColor              color.NRGBA
	BarUpColor                   color.NRGBA
	BarDownColor                 color.NRGBA
	AxesXtextColor               color.NRGBA
	AxesYtextColor               color.NRGBA
	QuoteDashColor               color.NRGBA
	QuoteDashPattern             []float32
	QuoteUpColor                 color.NRGBA
	QuoteDownColor               color.NRGBA
	QuoteTextColor               color.NRGBA
	HoverTextColor               color.NRGBA
	HoverBgColor                 color.NRGBA
}

func NewDarkPlotTheme() *PlotTheme {
	return &PlotTheme{
		AxesMarginMin:                DpPoint{X: 10, Y: 1},
		AxesMarginMax:                DpPoint{X: 30, Y: 10},
		SubPlotMarginY:               0,
		TextMargin:                   DpPoint{X: 7, Y: 7},
		AxesXfontSize:                17,
		AxesYfontSize:                17,
		DefaultPlotGrid:              DpPoint{X: 150, Y: 100},
		DefaultTimeUnitGrid:          4,
		AxesColor:                    color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		GridColor:                    color.NRGBA{R: 60, G: 60, B: 60, A: 255},
		CandleUnknownColor:           color.NRGBA{R: 100, G: 100, B: 100, A: 255},
		CandleUpColor:                color.NRGBA{R: 0, G: 255, B: 0, A: 255},
		CandleDownColor:              color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		CandleUpBorderColor:          color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		CandleDownBorderColor:        color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		UseBorderColorForCandleLines: false,
		DrawCandleUpBorder:           false,
		DrawCandleDownBorder:         false,
		BarUnknownColor:              color.NRGBA{R: 100, G: 100, B: 100, A: 255},
		BarUpColor:                   color.NRGBA{R: 0, G: 255, B: 0, A: 255},
		BarDownColor:                 color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		AxesXtextColor:               color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		AxesYtextColor:               color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		QuoteDashColor:               color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		QuoteDashPattern:             []float32{2, 10},
		QuoteUpColor:                 color.NRGBA{R: 0, G: 255, B: 0, A: 255},
		QuoteDownColor:               color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		QuoteTextColor:               color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		HoverTextColor:               color.NRGBA{R: 100, G: 255, B: 100, A: 255},
		HoverBgColor:                 color.NRGBA{R: 74, G: 74, B: 107, A: 255},
	}
}

func NewLightPlotTheme() *PlotTheme {
	return &PlotTheme{
		AxesMarginMin:                DpPoint{X: 10, Y: 1},
		AxesMarginMax:                DpPoint{X: 30, Y: 10},
		SubPlotMarginY:               0,
		TextMargin:                   DpPoint{X: 7, Y: 7},
		AxesXfontSize:                17,
		AxesYfontSize:                17,
		DefaultPlotGrid:              DpPoint{X: 150, Y: 100},
		DefaultTimeUnitGrid:          4,
		AxesColor:                    color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		GridColor:                    color.NRGBA{R: 230, G: 230, B: 230, A: 255},
		CandleUnknownColor:           color.NRGBA{R: 150, G: 150, B: 150, A: 255},
		CandleUpColor:                color.NRGBA{R: 0, G: 255, B: 0, A: 255},
		CandleDownColor:              color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		CandleUpBorderColor:          color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		CandleDownBorderColor:        color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		UseBorderColorForCandleLines: false,
		DrawCandleUpBorder:           false,
		DrawCandleDownBorder:         false,
		BarUnknownColor:              color.NRGBA{R: 150, G: 150, B: 150, A: 255},
		BarUpColor:                   color.NRGBA{R: 0, G: 255, B: 0, A: 255},
		BarDownColor:                 color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		AxesXtextColor:               color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		AxesYtextColor:               color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		QuoteDashColor:               color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		QuoteDashPattern:             []float32{2, 10},
		QuoteUpColor:                 color.NRGBA{R: 0, G: 255, B: 0, A: 255},
		QuoteDownColor:               color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		QuoteTextColor:               color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		HoverTextColor:               color.NRGBA{R: 100, G: 255, B: 100, A: 255},
		HoverBgColor:                 color.NRGBA{R: 174, G: 174, B: 207, A: 255},
	}
}

func (th *PlotTheme) GetCandleColors(isGreenCandle bool, consolidated bool) (candleColor, lineColor, borderColor color.NRGBA) {
	if isGreenCandle {
		borderColor = th.CandleUpBorderColor
	} else {
		borderColor = th.CandleDownBorderColor
	}

	if consolidated {
		if th.UseBorderColorForCandleLines {
			if isGreenCandle {
				candleColor = th.CandleUpColor
				lineColor = th.CandleUpBorderColor
			} else {
				candleColor = th.CandleDownColor
				lineColor = th.CandleDownBorderColor
			}
		} else {
			if isGreenCandle {
				candleColor = th.CandleUpColor
				lineColor = th.CandleUpColor
			} else {
				candleColor = th.CandleDownColor
				lineColor = th.CandleDownColor
			}
		}
	} else {
		candleColor = th.CandleUnknownColor
		lineColor = th.CandleUnknownColor
	}
	return
}
