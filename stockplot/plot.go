// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockplot

import (
	"image"
	"image/color"
	"math"
	"maystocks/indapi/candles"
	"maystocks/stockval"
	"maystocks/widgets"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Note that this is, by design, not a generic plotting library.
// It is specifically for stock market plots.
// X axis is always "time".

// X values are projected to "candle time units"
// We do not simply use unixtime to represent X values, because this causes
// problems with monthly candles (and daylight saving time).
// Monthly candles have non-constant time differences,
// due to different months having a different number of days.
type Plot struct {
	Theme               *widgets.PlotTheme
	gridX               unit.Dp
	zeroValueX          float64 // candle time units since Jan 1970 at zero position of plot
	valueGridX          float64
	pointerPressPos     f32.Point
	Sub                 []*SubPlot
	candleResolution    candles.CandleResolution
	requestFocus        bool
	previousPlotScaling stockval.PlotScaling
	frame               struct {
		totalPxSize      image.Point
		pxGridX          int
		axesMarginPxMin  image.Point
		axesMarginPxMax  image.Point
		subPlotMarginPxY int
		textMarginPx     image.Point
		textSizePx       image.Point
		nextTextSizePx   image.Point
		plotSizeX        float64
		xAxesTextPosY    int
	}
}

type PlotTag struct {
	a EventArea
	p *Plot
}

const MinGridDp = 2

func NewPlot(t *widgets.PlotTheme, r candles.CandleResolution, sx stockval.PlotScaling) *Plot {
	p := &Plot{
		Theme: t,
		Sub: []*SubPlot{
			{
				Theme:            t,
				pxSizeRatioY:     0.75,
				pxGridRatioY:     1,
				gridY:            t.DefaultPlotGrid.Y,
				valueGridY:       0.1,
				zoomValueY:       0.05,
				maxDecimalPlaces: 2,
				textPrecision:    2,
			},
			{
				Theme:            t,
				pxSizeRatioY:     0.25,
				pxGridRatioY:     0.5,
				gridY:            t.DefaultPlotGrid.Y,
				valueGridY:       1,
				zoomValueY:       1,
				maxDecimalPlaces: 0,
				textPrecision:    0,
				fixedZeroValueY:  true,
			},
		},
	}
	if sx.Grid > stockval.NearZero {
		p.gridX = sx.Grid
	} else {
		p.gridX = t.DefaultPlotGrid.X
	}
	if sx.ValueGrid > stockval.NearZero {
		p.valueGridX = sx.ValueGrid
	} else {
		p.valueGridX = t.DefaultTimeUnitGrid
	}
	p.setCandleResolution(r, true)
	return p
}

func (plot *Plot) setCandleResolution(r candles.CandleResolution, force bool) bool {
	if plot.candleResolution != r || force {
		plot.candleResolution = r
		// We do not need the exact duration here, just use a standard value and multiply to get base plot position.
		singleCandleDuration := r.GetDuration(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
		// TODO value grid should depend on resolution. i.e. daily candles should have a grid of 7 days
		// TODO Grid should be aligned and start at same interval. i.e. start on monday, or use 10 minutes base, or whatever.
		plot.zeroValueX = r.ConvertTimeToCandleUnits(time.Now().Add(singleCandleDuration * 2))
		// Regenerate base position and zoom during next rendering
		// TODO hardcoded subplot index
		plot.Sub[0].hasInitialCandleY = false
		plot.Sub[0].hasInitialRangeY = false
		plot.Sub[0].nextBaseValueY = 0
		plot.Sub[0].nextValueRangeY = 0
		return true
	}
	return false
}

func (plot *Plot) calcMinPos(subI int) image.Point {
	var marginY int
	if subI == 0 {
		marginY = plot.frame.axesMarginPxMin.Y
	} else {
		marginY = plot.frame.subPlotMarginPxY
	}
	return image.Point{
		X: plot.frame.axesMarginPxMin.X,
		Y: plot.Sub[subI].frame.basePos.Y + marginY,
	}
}

func (plot *Plot) calcMaxPos(subI int) image.Point {
	var marginY int
	if subI == len(plot.Sub)-1 {
		marginY = plot.frame.axesMarginPxMax.Y + plot.frame.textSizePx.Y
	} else {
		marginY = plot.frame.subPlotMarginPxY
	}
	return image.Point{
		X: plot.frame.totalPxSize.X - plot.frame.axesMarginPxMax.X - plot.frame.textSizePx.X,
		Y: plot.Sub[subI].frame.basePos.Y + plot.Sub[subI].frame.totalPxSize.Y - marginY,
	}
}

func (plot *Plot) calcPxPosRatioY(subI int) float64 {
	var pxPosRatioY float64
	// Add up all size ratios of plots above this one.
	for i := subI - 1; i >= 0; i-- {
		pxPosRatioY += plot.Sub[i].pxSizeRatioY
	}
	return pxPosRatioY
}

func (plot *Plot) calcFirstGridValueX() time.Time {
	return plot.candleResolution.GetNthCandleTime(plot.candleResolution.ConvertCandleUnitsToTime(math.Floor(plot.zeroValueX/plot.valueGridX)*plot.valueGridX), 0)
}

func (plot *Plot) calcProjectionVars(subI int) (proj projection) {
	maxPos := plot.Sub[subI].frame.maxPos
	// Projection f(v)=m*v+b
	// X values are increasing from bottom to top
	proj.mX = float64(plot.frame.pxGridX) / plot.valueGridX
	// Y values are decreasing from right to left
	proj.mY = -float64(plot.Sub[subI].frame.pxGridY) / plot.Sub[subI].valueGridY
	proj.bX = -proj.mX*plot.zeroValueX + float64(maxPos.X)
	proj.bY = -proj.mY*plot.Sub[subI].zeroValueY + float64(maxPos.Y)
	return
}

func (plot *Plot) optimiseGridX() {
	newGridX := plot.gridX
	newValueGridX := plot.valueGridX
	for ; newGridX/2 >= plot.Theme.DefaultPlotGrid.X*0.75 && newValueGridX >= 2; newGridX, newValueGridX = newGridX/2, newValueGridX/2 {
	}
	for ; newGridX*2 < plot.Theme.DefaultPlotGrid.X*1.25; newGridX, newValueGridX = newGridX*2, newValueGridX*2 {
	}
	plot.gridX = unit.Dp(math.Round(float64(newGridX)))
	plot.valueGridX = math.Round(newValueGridX)
}

func recordAxesLabelText(labelText string, c color.NRGBA, fontSize int, gtx layout.Context, th *material.Theme) (op.CallOp, image.Point) {
	macro := op.Record(gtx.Ops)
	lbl := material.Label(
		th,
		unit.Sp(fontSize),
		labelText,
	)
	lbl.Color = c
	lbl.Alignment = text.Start
	dims := lbl.Layout(gtx)
	return macro.Stop(), dims.Size
}

func (plot *Plot) paintXaxesText(gtx layout.Context, th *material.Theme) (maxTextSizeY int) {
	baseTime := plot.calcFirstGridValueX()
	posX := int(plot.Sub[0].frame.projection.getXpos(baseTime, plot.candleResolution))
	segmentsX := stockval.CalcNumSegments(posX, plot.frame.axesMarginPxMin.X, plot.frame.pxGridX)
	timeFormatStr := plot.candleResolution.FormatString()
	for i := 0; i < segmentsX; i++ {
		call, textSize := recordAxesLabelText(baseTime.Format(timeFormatStr), plot.Theme.AxesXtextColor, plot.Theme.AxesXfontSize, gtx, th)
		if textSize.Y > maxTextSizeY {
			maxTextSizeY = textSize.Y
		}
		stack := op.Offset(image.Point{X: posX - i*plot.frame.pxGridX - textSize.X/2, Y: plot.frame.xAxesTextPosY}).Push(gtx.Ops)
		// Run recorded drawing.
		call.Add(gtx.Ops)
		stack.Pop()
		baseTime = plot.candleResolution.GetNthCandleTime(baseTime, -int(plot.valueGridX))
	}
	return
}

func (plot *Plot) InitializeFrame(gtx layout.Context, r candles.CandleResolution) (candleResolutionChanged bool) {
	candleResolutionChanged = plot.setCandleResolution(r, false)
	plot.frame.totalPxSize = gtx.Constraints.Max
	plot.frame.axesMarginPxMin = plot.Theme.AxesMarginMin.Dp(gtx)
	plot.frame.axesMarginPxMax = plot.Theme.AxesMarginMax.Dp(gtx)
	plot.frame.subPlotMarginPxY = gtx.Dp(plot.Theme.SubPlotMarginY)
	plot.frame.textMarginPx = plot.Theme.TextMargin.Dp(gtx)
	plot.frame.pxGridX = gtx.Dp(plot.gridX)
	// Do not auto-scale down text size to avoid loops.
	if plot.frame.nextTextSizePx.X > 0 && plot.frame.nextTextSizePx.X > plot.frame.textSizePx.X {
		plot.frame.textSizePx.X = plot.frame.nextTextSizePx.X
		plot.frame.nextTextSizePx.X = 0
	}
	if plot.frame.nextTextSizePx.Y > 0 && plot.frame.nextTextSizePx.Y > plot.frame.textSizePx.Y {
		plot.frame.textSizePx.Y = plot.frame.nextTextSizePx.Y
		plot.frame.nextTextSizePx.Y = 0
	}
	plot.frame.plotSizeX = float64(plot.frame.totalPxSize.X - plot.frame.axesMarginPxMin.X - plot.frame.axesMarginPxMax.X - plot.frame.textSizePx.X)
	plot.frame.xAxesTextPosY = plot.frame.totalPxSize.Y - plot.frame.axesMarginPxMax.Y - plot.frame.textSizePx.Y + plot.frame.textMarginPx.Y
	for i, s := range plot.Sub {
		// Position and zoom of the subplot may be set asynchronously.
		// This uses plotSizeY, which is set below, but it is still called first due to other dependencies.
		s.updatePositionAndZoom(gtx, s.frame.plotSizeY)
		// Mind the order of updating frame values due to dependencies.
		s.frame.pxPosRatioY = plot.calcPxPosRatioY(i)
		s.frame.basePos.X = 0
		s.frame.basePos.Y = int(float64(plot.frame.totalPxSize.Y) * s.frame.pxPosRatioY)
		s.frame.totalPxSize.X = plot.frame.totalPxSize.X
		s.frame.totalPxSize.Y = int(float64(plot.frame.totalPxSize.Y) * s.pxSizeRatioY)
		s.frame.pxGridY = gtx.Dp(s.gridY)
		s.frame.minPos = plot.calcMinPos(i)
		s.frame.maxPos = plot.calcMaxPos(i)
		s.frame.plotSizeY = float64(s.frame.totalPxSize.Y) - float64(plot.frame.axesMarginPxMin.Y+plot.frame.axesMarginPxMax.Y+plot.frame.textSizePx.Y)
		s.frame.yAxesTextPosX = plot.frame.totalPxSize.X - plot.frame.axesMarginPxMax.X - plot.frame.textSizePx.X + plot.frame.textMarginPx.X
		s.frame.projection = plot.calcProjectionVars(i)
	}
	plot.handleInput(gtx)
	plot.registerInputOps(gtx.Ops)
	return
}

func (plot *Plot) registerInputOps(ops *op.Ops) {
	xAxisArea := clip.Rect(image.Rectangle{
		Min: image.Point{X: plot.frame.axesMarginPxMin.X, Y: plot.frame.totalPxSize.Y - plot.frame.axesMarginPxMax.Y - plot.frame.textSizePx.Y},
		Max: image.Point{X: plot.frame.totalPxSize.X - plot.frame.axesMarginPxMax.X - plot.frame.textSizePx.X, Y: plot.frame.totalPxSize.Y}}).Push(ops)
	pointer.InputOp{
		Tag:   PlotTag{a: EventAreaXaxis, p: plot},
		Types: pointer.Press | pointer.Release | pointer.Drag,
	}.Add(ops)
	pointer.CursorColResize.Add(ops)
	if plot.requestFocus {
		key.FocusOp{Tag: PlotTag{a: EventAreaXaxis, p: plot}}.Add(ops)
		plot.requestFocus = false
	}
	xAxisArea.Pop()
	// pointer input per subplot
	for _, s := range plot.Sub {
		subArea := clip.Rect(image.Rectangle{Min: s.frame.minPos, Max: s.frame.maxPos}).Push(ops)
		pointer.InputOp{
			Tag:   SubPlotTag{a: EventAreaPlot, s: s},
			Types: pointer.Press | pointer.Drag | pointer.Scroll,
			ScrollBounds: image.Rectangle{
				Min: image.Point{
					X: 0,
					Y: math.MinInt,
				},
				Max: image.Point{
					X: 0,
					Y: math.MaxInt,
				},
			},
		}.Add(ops)
		subArea.Pop()

		yAxisArea := clip.Rect(image.Rectangle{
			Min: image.Point{X: s.frame.maxPos.X, Y: s.frame.minPos.Y},
			Max: s.frame.basePos.Add(s.frame.totalPxSize)}).Push(ops)
		pointer.InputOp{
			Tag:   SubPlotTag{a: EventAreaYaxis, s: s},
			Types: pointer.Press | pointer.Release | pointer.Drag,
		}.Add(ops)
		pointer.CursorRowResize.Add(ops)
		yAxisArea.Pop()
	}
}

func (plot *Plot) handleInput(gtx layout.Context) {
	// X axis zooming
	for _, gtxEvent := range gtx.Events(PlotTag{a: EventAreaXaxis, p: plot}) {
		switch e := gtxEvent.(type) {
		case pointer.Event:
			if e.Type == pointer.Press {
				plot.pointerPressPos = e.Position // TODO maybe support multiple pointers
			} else if e.Type == pointer.Drag {
				posDelta := plot.pointerPressPos.Sub(e.Position)
				dpDelta := gtx.Metric.PxToDp(int(posDelta.X)) / 5
				if dpDelta != 0 {
					plot.gridX = plot.gridX + dpDelta
					if plot.gridX < MinGridDp {
						plot.gridX = MinGridDp
					}
					plot.pointerPressPos = e.Position
					plot.optimiseGridX()
					plot.frame.pxGridX = gtx.Dp(plot.gridX)
				}
			}
		}
	}
	// subplot specific events
	for _, s := range plot.Sub {
		for _, gtxEvent := range gtx.Events(SubPlotTag{a: EventAreaPlot, s: s}) {
			switch e := gtxEvent.(type) {
			case pointer.Event:
				plot.requestFocus = true
				if e.Type == pointer.Press {
					plot.pointerPressPos = e.Position
				} else if e.Type == pointer.Drag {
					posDelta := plot.pointerPressPos.Sub(e.Position)
					plot.zeroValueX += plot.valueGridX / float64(plot.frame.pxGridX) * float64(posDelta.X)
					if !s.fixedZeroValueY {
						s.zeroValueY -= s.valueGridY / float64(s.frame.pxGridY) * float64(posDelta.Y)
						if s.zeroValueY < 0 {
							s.zeroValueY = 0
						}
					}
					plot.pointerPressPos = e.Position
				} else if e.Type == pointer.Scroll {
					var zoom unit.Dp
					if e.Scroll.Y < 0 {
						zoom = -10
					} else {
						zoom = 10
					}
					plot.gridX += zoom
					if plot.gridX < MinGridDp {
						plot.gridX = MinGridDp
					}
					plot.optimiseGridX()
					plot.frame.pxGridX = gtx.Dp(plot.gridX)
				}
			}
		}

		for _, gtxEvent := range gtx.Events(SubPlotTag{a: EventAreaYaxis, s: s}) {
			switch e := gtxEvent.(type) {
			case pointer.Event:
				plot.requestFocus = true
				if e.Type == pointer.Press {
					plot.pointerPressPos = e.Position
				} else if e.Type == pointer.Drag {
					posDelta := plot.pointerPressPos.Sub(e.Position)
					s.gridY += gtx.Metric.PxToDp(int(posDelta.Y)) / 2
					if s.gridY < MinGridDp {
						s.gridY = MinGridDp
					}
					plot.pointerPressPos = e.Position
					s.frame.pxGridY = gtx.Dp(s.gridY)
					s.nextValueRangeY = s.calcYvalueRange()
					op.InvalidateOp{}.Add(gtx.Ops)
				}
			}
		}
	}
}

func (plot *Plot) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	for _, s := range plot.Sub {
		s.paintGrid(
			plot.candleResolution,
			plot.calcFirstGridValueX(),
			plot.frame.pxGridX,
			gtx,
		)
		s.paintAxes(
			gtx,
		)
		maxTextSizeX := s.paintYaxesText(
			gtx,
			th,
		)
		if maxTextSizeX > plot.frame.nextTextSizePx.X {
			plot.frame.nextTextSizePx.X = maxTextSizeX
		}
	}
	maxTextSizeY := plot.paintXaxesText(
		gtx,
		th,
	)
	if maxTextSizeY > plot.frame.nextTextSizePx.Y {
		plot.frame.nextTextSizePx.Y = maxTextSizeY
	}
	return layout.Dimensions{Size: plot.frame.totalPxSize}
}

func (plot *Plot) calcXvalueRange() float64 {
	return plot.frame.plotSizeX / float64(plot.frame.pxGridX) * plot.valueGridX
}

// Call from same goroutine as Layout
func (plot *Plot) GetCandleRange() (startTime time.Time, endTime time.Time, resolution candles.CandleResolution) {
	endTime = plot.candleResolution.ConvertCandleUnitsToTime(plot.zeroValueX)
	minVal := plot.zeroValueX - plot.calcXvalueRange()
	startTime = plot.candleResolution.ConvertCandleUnitsToTime(minVal)
	resolution = plot.candleResolution
	return
}

// Call from same goroutine as Layout
func (plot *Plot) GetPlotScalingX() (stockval.PlotScaling, bool) {
	newPlotScaling := stockval.PlotScaling{
		Grid:      plot.gridX,
		ValueGrid: plot.valueGridX,
	}
	changed := plot.previousPlotScaling != newPlotScaling
	if changed {
		plot.previousPlotScaling = newPlotScaling
	}
	return newPlotScaling, changed
}
