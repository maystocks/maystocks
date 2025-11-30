// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockplot

import (
	"image"
	"image/color"
	"log"
	"math"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/stockval"
	"maystocks/widgets"
	"strconv"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	// The builtin gio stroke has a lot of issues, one being that horizontal and vertical lines
	// may have different thickness, even if the same width is specified.
	// We use the "x/stroke" extension instead, it works like a charm.
	"gioui.org/x/stroke"
)

// All subplots of a plot have the same X values but can have different Y values
type SubPlot struct {
	Type              indapi.SubPlotType
	Theme             *widgets.PlotTheme
	Indicators        []indapi.IndicatorData
	gridY             unit.Dp
	zeroValueY        float64 // Y value at zero position of plot
	hasInitialCandleY bool
	hasInitialQuoteY  bool
	hasInitialRangeY  bool
	nextBaseValueY    float64
	nextValueRangeY   float64
	pxSizeRatioY      float64
	SubPlotTemplate
	frame struct {
		basePos                         image.Point
		totalPxSize                     image.Point
		minPos                          image.Point // plot area
		maxPos                          image.Point // plot area
		pxGridY                         int
		pxPosRatioY                     float64
		plotSizeY                       float64
		printFormat                     printFormat
		yAxesTextPosX                   int
		projection                      projection
		labelValues                     []float64
		gridSegments                    []stroke.Segment
		lineSegments                    []stroke.Segment
		greenCandleBorderSegments       []stroke.Segment
		greenCandleLineSegments         []stroke.Segment
		greenCandleSegments             []stroke.Segment
		redCandleBorderSegments         []stroke.Segment
		redCandleLineSegments           []stroke.Segment
		redCandleSegments               []stroke.Segment
		unsureGreenCandleBorderSegments []stroke.Segment
		unsureGreenCandleLineSegments   []stroke.Segment
		unsureGreenCandleSegments       []stroke.Segment
		unsureRedCandleBorderSegments   []stroke.Segment
		unsureRedCandleLineSegments     []stroke.Segment
		unsureRedCandleSegments         []stroke.Segment
		greenVolumeSegments             []stroke.Segment
		redVolumeSegments               []stroke.Segment
		unsureGreenVolumeSegments       []stroke.Segment
		unsureRedVolumeSegments         []stroke.Segment
	}
}

type SubPlotTemplate struct {
	pxBaseRatioY     float64
	pxGridRatioY     float64
	valueGridY       float64
	zoomValueY       float64
	maxDecimalPlaces int
	textPrecision    int
	fixedZeroValueY  bool
}

type printFormat int

const (
	printFormatDefault printFormat = iota
	printFormatThousands
	printFormatMillions
	printFormatBillions
)

type SubPlotTag struct {
	a EventArea
	s *SubPlot
}

type projection struct {
	mX float64
	mY float64
	bX float64
	bY float64
}

func (proj projection) getXpos(t time.Time, r candles.CandleResolution) float64 {
	return proj.mX*r.ConvertTimeToCandleUnits(t) + proj.bX
}

func (proj projection) getYpos(v float64) float64 {
	return proj.mY*v + proj.bY
}

func (sub *SubPlot) calcFirstGridValueY() float64 {
	return math.Ceil(sub.zeroValueY/sub.valueGridY) * sub.valueGridY
}

func (sub *SubPlot) calcYvalueRange() float64 {
	return sub.frame.plotSizeY / float64(sub.frame.pxGridY) * sub.valueGridY
}

// Position and zoom of plot can be updated during rendering.
// This prepares values for the next frame, so call first before using grid values.
func (sub *SubPlot) updatePositionAndZoom(gtx layout.Context, yPlotSize float64) {
	if sub.nextValueRangeY > stockval.NearZero {
		// TODO draw animation if zoom is changed
		// Calculate some base value for number of segments per subplot, will only be used as guidance.
		numSegments := math.Ceil(10/5*sub.pxSizeRatioY) * 5
		var decimalBase float64
		var minValueGrid float64
		if sub.nextValueRangeY >= 10 {
			// This helps to avoid unintuitive value differences.
			decimalBase = 1 / math.Pow10(stockval.CountDigits(int64(sub.nextValueRangeY))-1)
		} else if sub.nextValueRangeY >= 1 {
			decimalBase = 10
		} else {
			decimalBase = math.Pow10(sub.maxDecimalPlaces)
		}
		minValueGrid = 1 / decimalBase
		nextValueGrid := math.Ceil((sub.nextValueRangeY/numSegments)*decimalBase) / decimalBase
		if nextValueGrid <= minValueGrid {
			nextValueGrid = minValueGrid
		} else {
			// The value grid calculated up to this point will very likely not be intuitive, it might be a price
			// difference like 0.3$. In order to avoid strange differences, we round to some value.
			if nextValueGrid*decimalBase > 5 {
				diff := math.Mod(nextValueGrid*decimalBase, 10)
				if diff > stockval.NearZero {
					nextValueGrid += (10 - diff) * (1 / decimalBase)
				}
			} else if nextValueGrid*decimalBase > 2 {
				diff := math.Mod(nextValueGrid*decimalBase, 5)
				if diff > stockval.NearZero {
					nextValueGrid += (5 - diff) * (1 / decimalBase)
				}
			} else {
				diff := math.Mod(nextValueGrid*decimalBase, 2)
				if diff > stockval.NearZero {
					nextValueGrid += (2 - diff) * (1 / decimalBase)
				}
			}
		}
		sub.valueGridY = nextValueGrid
		sub.gridY = unit.Dp((yPlotSize / (sub.nextValueRangeY / nextValueGrid)) / float64(gtx.Metric.PxPerDp))
		if sub.gridY < MinGridDp {
			sub.gridY = MinGridDp
		}
		sub.optimiseGridY()
		sub.nextValueRangeY = 0
	}
	if sub.nextBaseValueY > stockval.NearZero {
		// The zero value is initialized one grid below the base value.
		sub.zeroValueY = sub.nextBaseValueY - sub.valueGridY
		log.Printf("Initial value: %f ZeroValue: %f", sub.nextBaseValueY, sub.zeroValueY)
		sub.nextBaseValueY = 0
	}
}

func (sub *SubPlot) optimiseGridY() {
	newGridY := sub.gridY
	newValueGridY := sub.valueGridY
	for ; newGridY/2 >= sub.Theme.DefaultPlotGrid.Y*0.75*unit.Dp(sub.pxGridRatioY) && newValueGridY >= 2; newGridY, newValueGridY = newGridY/2, newValueGridY/2 {
	}
	for ; newGridY*2 < sub.Theme.DefaultPlotGrid.Y*1.25*unit.Dp(sub.pxGridRatioY); newGridY, newValueGridY = newGridY*2, newValueGridY*2 {
	}
	sub.gridY = unit.Dp(math.Round(float64(newGridY)))
	decimalBase := math.Pow10(sub.maxDecimalPlaces)
	sub.valueGridY = math.Round(newValueGridY*decimalBase) / decimalBase
}

func (sub *SubPlot) formatYlabel(value float64) string {
	switch sub.frame.printFormat {
	case printFormatBillions:
		return strconv.FormatFloat(value/1000000000, 'f', sub.textPrecision, 64) + "b"
	case printFormatMillions:
		return strconv.FormatFloat(value/1000000, 'f', sub.textPrecision, 64) + "m"
	case printFormatThousands:
		return strconv.FormatFloat(value/1000, 'f', sub.textPrecision, 64) + "k"
	default:
		return strconv.FormatFloat(value, 'f', sub.textPrecision, 64)
	}
}

func (sub *SubPlot) paintYaxesText(gtx layout.Context, th *material.Theme) (maxTextSizeX int) {
	baseValue := sub.calcFirstGridValueY()
	posY := int(sub.frame.projection.getYpos(baseValue))
	sub.calculateLabelValues(posY, baseValue)
	sub.determineLabelPrintFormat()
	var labelText string
	for i, v := range sub.frame.labelValues {
		newLabelText := sub.formatYlabel(v)
		if newLabelText == labelText {
			continue // do not print text twice if it is unchanged due to precision
		}
		labelText = newLabelText
		// Record drawing to pre-calculate text size.
		call, textSize := recordAxesLabelText(labelText, sub.Theme.AxesYtextColor, sub.Theme.AxesYfontSize, gtx, th)
		if textSize.X > maxTextSizeX {
			maxTextSizeX = textSize.X
		}
		stack := op.Offset(image.Point{X: sub.frame.yAxesTextPosX, Y: posY - i*sub.frame.pxGridY - textSize.Y/2}).Push(gtx.Ops)
		// Run recorded drawing.
		call.Add(gtx.Ops)
		stack.Pop()
	}
	return
}

func (sub *SubPlot) calculateLabelValues(posY int, baseValue float64) {
	segmentsY := stockval.CalcNumSegments(posY, sub.frame.minPos.Y, sub.frame.pxGridY)
	labelValues := sub.frame.labelValues[:0]
	for i := 0; i < segmentsY; i++ {
		labelValue := baseValue + float64(i)*sub.valueGridY
		// we do not want negative zero on our label
		if labelValue < 0 && labelValue > -stockval.NearZero {
			labelValue = 0
		}
		labelValues = append(labelValues, labelValue)
	}
	sub.frame.labelValues = labelValues
}

func (sub *SubPlot) determineLabelPrintFormat() {
	printBillions := true
	printMillions := true
	printThousands := true
	for i, v := range sub.frame.labelValues {
		labelValueI := int64(v)

		// Check whether all values are billions, millions or thousands.
		if (i != 0 && labelValueI/1000000000 == 0) || labelValueI%1000000000 != 0 {
			printBillions = false
		}
		if (i != 0 && labelValueI/1000000 == 0) || labelValueI%1000000 != 0 {
			printMillions = false
		}
		if (i != 0 && labelValueI/1000 == 0) || labelValueI%1000 != 0 {
			printThousands = false
		}
	}
	if printBillions {
		sub.frame.printFormat = printFormatBillions
	} else if printMillions {
		sub.frame.printFormat = printFormatMillions
	} else if printThousands {
		sub.frame.printFormat = printFormatThousands
	}
}

func (sub *SubPlot) paintAxes(gtx layout.Context) {
	minPos := sub.frame.minPos
	maxPos := sub.frame.maxPos
	var path stroke.Path
	path.Segments = []stroke.Segment{
		stroke.MoveTo(f32.Pt(float32(maxPos.X), float32(minPos.Y))),
		stroke.LineTo(f32.Pt(float32(maxPos.X), float32(maxPos.Y))),
		stroke.MoveTo(f32.Pt(float32(minPos.X), float32(maxPos.Y))),
		stroke.LineTo(f32.Pt(float32(maxPos.X), float32(maxPos.Y))),
	}
	area := stroke.Stroke{Path: path, Width: 1}.Op(gtx.Ops)
	paint.FillShape(gtx.Ops, sub.Theme.AxesColor, area)
}

func (sub *SubPlot) paintGrid(r candles.CandleResolution, firstGridValueX time.Time, pxGridX int, gtx layout.Context) {
	minPos := sub.frame.minPos
	maxPos := sub.frame.maxPos
	baseX := firstGridValueX
	baseY := sub.calcFirstGridValueY()
	posX := int(sub.frame.projection.getXpos(baseX, r))
	posY := int(sub.frame.projection.getYpos(baseY))

	segmentsX := stockval.CalcNumSegments(posX, minPos.X, pxGridX)
	segmentsY := stockval.CalcNumSegments(posY, minPos.Y, sub.frame.pxGridY)
	var path stroke.Path
	path.Segments = sub.frame.gridSegments[:0]
	for i := 0; i < segmentsX; i++ {
		path.Segments = append(path.Segments, stroke.MoveTo(f32.Pt(float32(posX-i*pxGridX), float32(minPos.Y))))
		path.Segments = append(path.Segments, stroke.LineTo(f32.Pt(float32(posX-i*pxGridX), float32(maxPos.Y))))
	}
	for i := 0; i < segmentsY; i++ {
		path.Segments = append(path.Segments, stroke.MoveTo(f32.Pt(float32(minPos.X), float32(posY-i*sub.frame.pxGridY))))
		path.Segments = append(path.Segments, stroke.LineTo(f32.Pt(float32(maxPos.X), float32(posY-i*sub.frame.pxGridY))))
	}
	sub.frame.gridSegments = path.Segments
	area := stroke.Stroke{Path: path, Width: float32(gtx.Dp(1))}.Op(gtx.Ops)
	paint.FillShape(gtx.Ops, sub.Theme.GridColor, area)
}

func (sub *SubPlot) plotLineSegment(t time.Time, value float64, r candles.CandleResolution, pxPos, pyPos *float64,
	pxPosI, pyPosI *int, first bool, clipRect image.Rectangle, path *stroke.Path) {
	xPos := sub.frame.projection.getXpos(t, r)
	xPosI := int(xPos)
	yPos := sub.frame.projection.getYpos(value)
	yPosI := int(yPos)
	if !first {
		// Performance: We only draw a line if we hit a different pixel.
		if *pxPosI != xPosI || *pyPosI != yPosI {
			// Performance: Check if one of the positions is within visible plot range.
			// This will also implicitly be done by clipping to the plot area later,
			// but we are filtering here to avoid allocations in case there is a lot of data.
			if !((xPosI < clipRect.Min.X && *pxPosI < clipRect.Min.X) || (xPosI > clipRect.Max.X && *pxPosI > clipRect.Max.X) ||
				(yPosI < clipRect.Min.Y && *pyPosI < clipRect.Min.Y) || (yPosI > clipRect.Max.Y && *pyPosI > clipRect.Max.Y)) {
				path.Segments = append(path.Segments, stroke.LineTo(f32.Pt(float32(xPos), float32(yPos))))
			}
		}
	} else {
		path.Segments = append(path.Segments, stroke.MoveTo(f32.Pt(float32(xPos), float32(yPos))))
	}
	*pxPos = xPos
	*pxPosI = xPosI
	*pyPos = yPos
	*pyPosI = yPosI
}

func (sub *SubPlot) PlotLine(timestamps []time.Time, data []float64, maxValue *float64, r candles.CandleResolution, c color.NRGBA, gtx layout.Context) {
	dataSize := len(data)
	if dataSize <= 1 {
		return
	}
	var path stroke.Path
	// Reuse line segment buffer from previous frame. This may grow a lot, but considerably improves performance.
	path.Segments = sub.frame.lineSegments[:0]
	clipRect := image.Rectangle{Min: sub.frame.minPos, Max: sub.frame.maxPos}

	var pxPos float64 = -1
	var pxPosI int = -1
	var pyPos float64 = -1
	var pyPosI int = -1
	for i, t := range timestamps {
		sub.plotLineSegment(t, data[i], r, &pxPos, &pyPos, &pxPosI, &pyPosI, i == 0, clipRect, &path)
		if data[i] > *maxValue {
			*maxValue = data[i]
		}
	}

	// Only draw within the plot area.
	defer clip.Rect(clipRect).Push(gtx.Ops).Pop()
	// Draw all data with a single stroke.
	sub.frame.lineSegments = path.Segments
	pathArea := stroke.Stroke{Path: path, Width: 1}.Op(gtx.Ops)
	paint.FillShape(gtx.Ops, c, pathArea)
}

func (sub *SubPlot) resetCandleSegments() {
	sub.frame.greenCandleBorderSegments = sub.frame.greenCandleBorderSegments[:0]
	sub.frame.greenCandleLineSegments = sub.frame.greenCandleLineSegments[:0]
	sub.frame.greenCandleSegments = sub.frame.greenCandleSegments[:0]
	sub.frame.redCandleBorderSegments = sub.frame.redCandleBorderSegments[:0]
	sub.frame.redCandleLineSegments = sub.frame.redCandleLineSegments[:0]
	sub.frame.redCandleSegments = sub.frame.redCandleSegments[:0]
	sub.frame.unsureGreenCandleBorderSegments = sub.frame.unsureGreenCandleBorderSegments[:0]
	sub.frame.unsureGreenCandleLineSegments = sub.frame.unsureGreenCandleLineSegments[:0]
	sub.frame.unsureGreenCandleSegments = sub.frame.unsureGreenCandleSegments[:0]
	sub.frame.unsureRedCandleBorderSegments = sub.frame.unsureRedCandleBorderSegments[:0]
	sub.frame.unsureRedCandleLineSegments = sub.frame.unsureRedCandleLineSegments[:0]
	sub.frame.unsureRedCandleSegments = sub.frame.unsureRedCandleSegments[:0]
}

func (sub *SubPlot) strokeCandleSegments(gtx layout.Context, seg []stroke.Segment, lineWidth float32, lineColor color.NRGBA) {
	if len(seg) == 0 {
		return
	}
	var path stroke.Path
	path.Segments = seg
	paint.FillShape(
		gtx.Ops,
		lineColor,
		stroke.Stroke{Path: path, Width: lineWidth, Cap: stroke.FlatCap}.Op(gtx.Ops),
	)
}

func (sub *SubPlot) UpdateIndicators(data *stockval.CandlePlotData) {
	for _, ind := range sub.Indicators {
		ind.Update(data.Resolution, &data.PlotData)
	}
}

func (sub *SubPlot) Plot(data *stockval.CandlePlotData, quote stockval.QuoteData, gtx layout.Context, th *material.Theme) {
	var maxIndicatorValue float64
	switch sub.Type {
	case indapi.SubPlotTypePrice:
		sub.plotCandles(
			data,
			gtx,
		)
		sub.plotQuoteLine(
			quote,
			gtx,
			th,
		)
		for _, ind := range sub.Indicators {
			ind.Plot(sub, &maxIndicatorValue, sub.Theme.DefaultIndicatorColor, gtx)
		}
	case indapi.SubPlotTypeVolume:
		sub.plotVolumeBars(
			data,
			gtx,
		)
	case indapi.SubPlotTypeIndicator:
		for _, ind := range sub.Indicators {
			ind.Plot(sub, &maxIndicatorValue, sub.Theme.DefaultIndicatorColor, gtx)
		}
		sub.autoZoomGenericY(maxIndicatorValue, gtx)
	}
}

func (sub *SubPlot) plotCandles(data *stockval.CandlePlotData, gtx layout.Context) {
	var minPrice, maxPrice float64

	sub.resetCandleSegments()
	// Only draw within the plot area.
	clipRect := image.Rectangle{Min: sub.frame.minPos, Max: sub.frame.maxPos}
	defer clip.Rect(clipRect).Push(gtx.Ops).Pop()

	data.DataMutex.RLock()
	hasData := len(data.Data) > 0
	for _, d := range data.Data {
		l, _ := d.LowPrice.Float64()
		h, _ := d.HighPrice.Float64()
		o, _ := d.OpenPrice.Float64()
		c, _ := d.ClosePrice.Float64()
		sub.plotSingleCandle(l, h, o, c, d.Timestamp, data.Resolution, true, &minPrice, &maxPrice, clipRect, gtx)
	}
	data.DataMutex.RUnlock()

	data.RealtimeData.DataMutex.RLock()
	for i, d := range data.RealtimeData.Data {
		if data.HasValidRealtimePrices(i) {
			l, _ := d.LowPrice.Float64()
			h, _ := d.HighPrice.Float64()
			o, _ := d.OpenPrice.Float64()
			c, _ := d.ClosePrice.Float64()
			sub.plotSingleCandle(l, h, o, c, d.Timestamp, data.Resolution, data.RealtimeData.OpenConsolidated[i], &minPrice, &maxPrice, clipRect, gtx)
		}
	}
	data.RealtimeData.DataMutex.RUnlock()
	candleWidth, lineWidth, borderWidth := getCandleWidth(sub.frame.projection.mX, gtx.Dp(1))
	candleColor, lineColor, borderColor := sub.Theme.GetCandleColors(true, true)
	var actualBorderWidth int
	if len(sub.frame.greenCandleBorderSegments) > 0 || len(sub.frame.redCandleBorderSegments) > 0 ||
		len(sub.frame.unsureGreenCandleBorderSegments) > 0 || len(sub.frame.unsureRedCandleBorderSegments) > 0 {
		actualBorderWidth = borderWidth * 2
	}
	sub.strokeCandleSegments(gtx, sub.frame.greenCandleLineSegments, float32(lineWidth), lineColor)
	sub.strokeCandleSegments(gtx, sub.frame.greenCandleBorderSegments, float32(borderWidth), borderColor)
	sub.strokeCandleSegments(gtx, sub.frame.greenCandleSegments, float32(candleWidth-actualBorderWidth), candleColor)
	candleColor, lineColor, borderColor = sub.Theme.GetCandleColors(false, true)
	sub.strokeCandleSegments(gtx, sub.frame.redCandleLineSegments, float32(lineWidth), lineColor)
	sub.strokeCandleSegments(gtx, sub.frame.redCandleBorderSegments, float32(borderWidth), borderColor)
	sub.strokeCandleSegments(gtx, sub.frame.redCandleSegments, float32(candleWidth-actualBorderWidth), candleColor)
	candleColor, lineColor, borderColor = sub.Theme.GetCandleColors(true, false)
	sub.strokeCandleSegments(gtx, sub.frame.unsureGreenCandleLineSegments, float32(lineWidth), lineColor)
	sub.strokeCandleSegments(gtx, sub.frame.unsureGreenCandleBorderSegments, float32(borderWidth), borderColor)
	sub.strokeCandleSegments(gtx, sub.frame.unsureGreenCandleSegments, float32(candleWidth-actualBorderWidth), candleColor)
	candleColor, lineColor, borderColor = sub.Theme.GetCandleColors(false, false)
	sub.strokeCandleSegments(gtx, sub.frame.unsureRedCandleLineSegments, float32(lineWidth), lineColor)
	sub.strokeCandleSegments(gtx, sub.frame.unsureRedCandleBorderSegments, float32(borderWidth), borderColor)
	sub.strokeCandleSegments(gtx, sub.frame.unsureRedCandleSegments, float32(candleWidth-actualBorderWidth), candleColor)

	if hasData && minPrice > stockval.NearZero && maxPrice > stockval.NearZero {
		var invalidate bool
		if !sub.hasInitialCandleY {
			sub.nextBaseValueY = minPrice
			sub.hasInitialCandleY = true
			invalidate = true
		}
		if !sub.hasInitialRangeY {
			sub.nextValueRangeY = sub.getValueRange(maxPrice - minPrice)
			sub.hasInitialRangeY = true
			invalidate = true
		}
		if invalidate {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (sub *SubPlot) plotSingleCandle(l, h, o, c float64, t time.Time, r candles.CandleResolution, consolidated bool,
	minPrice, maxPrice *float64, clipRect image.Rectangle, gtx layout.Context) {
	candleWidth, _, borderWidth := getCandleWidth(sub.frame.projection.mX, gtx.Dp(1))
	xPos := sub.frame.projection.getXpos(t, r)
	y1Pos := sub.frame.projection.getYpos(l)
	y2Pos := sub.frame.projection.getYpos(h)
	if math.Round(y1Pos) == math.Round(y2Pos) {
		y2Pos++ // Stroke does not draw zero length lines, see https://github.com/andybalholm/stroke/issues/3
	}
	y3Pos := sub.frame.projection.getYpos(o)
	y4Pos := sub.frame.projection.getYpos(c)

	// Performance: Skip entries where both positions are outside of the visible plot range
	// (both on the same side of the plot only).
	// This will also implicitly be done by clipping to the plot area,
	// but we are filtering here to avoid paint operations in case there is a lot of data.
	if !(int(xPos)+candleWidth/2 < clipRect.Min.X || int(xPos)-candleWidth/2 > clipRect.Max.X) {
		// Update min/max price even if candles are not visible considering Y axis
		if *minPrice < stockval.NearZero || l < *minPrice {
			*minPrice = l
		}
		if h > *maxPrice {
			*maxPrice = h
		}
		if !((int(y1Pos) < clipRect.Min.Y && int(y2Pos) < clipRect.Min.Y) || (int(y1Pos) > clipRect.Max.Y && int(y2Pos) > clipRect.Max.Y)) {
			isGreenCandle := stockval.IsGreenCandle(o, c)

			// Draw line first
			seg1 := stroke.MoveTo(f32.Pt(float32(xPos), float32(y1Pos)))
			seg2 := stroke.LineTo(f32.Pt(float32(xPos), float32(y2Pos)))
			if isGreenCandle {
				if consolidated {
					sub.frame.greenCandleLineSegments = append(sub.frame.greenCandleLineSegments, seg1)
					sub.frame.greenCandleLineSegments = append(sub.frame.greenCandleLineSegments, seg2)
				} else {
					sub.frame.unsureGreenCandleLineSegments = append(sub.frame.unsureGreenCandleLineSegments, seg1)
					sub.frame.unsureGreenCandleLineSegments = append(sub.frame.unsureGreenCandleLineSegments, seg2)
				}
			} else {
				if consolidated {
					sub.frame.redCandleLineSegments = append(sub.frame.redCandleLineSegments, seg1)
					sub.frame.redCandleLineSegments = append(sub.frame.redCandleLineSegments, seg2)
				} else {
					sub.frame.unsureRedCandleLineSegments = append(sub.frame.unsureRedCandleLineSegments, seg1)
					sub.frame.unsureRedCandleLineSegments = append(sub.frame.unsureRedCandleLineSegments, seg2)
				}
			}

			// Draw candle using a minimum height of 1 px
			if math.Round(y4Pos) == math.Round(y3Pos) {
				y4Pos--
			}
			// clip.Rect does not work well to draw candles, because it has integer resolution
			// and will lead to jumping of candles during scrolling.
			// Therefore, we are drawing candles as "thick lines" with a flat cap.
			var borderSize float64
			if (isGreenCandle && sub.Theme.DrawCandleUpBorder) || (!isGreenCandle && sub.Theme.DrawCandleDownBorder) {
				borderSize = float64(borderWidth)
				if isGreenCandle {
					bor1 := stroke.MoveTo(f32.Pt(float32(xPos), float32(y4Pos)))
					bor2 := stroke.LineTo(f32.Pt(float32(xPos), float32(y3Pos)))
					if consolidated {
						sub.frame.greenCandleBorderSegments = append(sub.frame.greenCandleBorderSegments, bor1)
						sub.frame.greenCandleBorderSegments = append(sub.frame.greenCandleBorderSegments, bor2)
					} else {
						sub.frame.unsureGreenCandleBorderSegments = append(sub.frame.unsureGreenCandleBorderSegments, bor1)
						sub.frame.unsureGreenCandleBorderSegments = append(sub.frame.unsureGreenCandleBorderSegments, bor2)
					}
				} else {
					bor1 := stroke.MoveTo(f32.Pt(float32(xPos), float32(y3Pos)))
					bor2 := stroke.LineTo(f32.Pt(float32(xPos), float32(y4Pos)))
					if consolidated {
						sub.frame.redCandleBorderSegments = append(sub.frame.redCandleBorderSegments, bor1)
						sub.frame.redCandleBorderSegments = append(sub.frame.redCandleBorderSegments, bor2)
					} else {
						sub.frame.unsureRedCandleBorderSegments = append(sub.frame.unsureRedCandleBorderSegments, bor1)
						sub.frame.unsureRedCandleBorderSegments = append(sub.frame.unsureRedCandleBorderSegments, bor2)
					}
				}
			}

			if isGreenCandle {
				bor1 := stroke.MoveTo(f32.Pt(float32(xPos), float32(y4Pos+borderSize)))
				bor2 := stroke.LineTo(f32.Pt(float32(xPos), float32(y3Pos-borderSize)))
				if consolidated {
					sub.frame.greenCandleSegments = append(sub.frame.greenCandleSegments, bor1)
					sub.frame.greenCandleSegments = append(sub.frame.greenCandleSegments, bor2)
				} else {
					sub.frame.unsureGreenCandleSegments = append(sub.frame.unsureGreenCandleSegments, bor1)
					sub.frame.unsureGreenCandleSegments = append(sub.frame.unsureGreenCandleSegments, bor2)
				}
			} else {
				bor1 := stroke.MoveTo(f32.Pt(float32(xPos), float32(y3Pos+borderSize)))
				bor2 := stroke.LineTo(f32.Pt(float32(xPos), float32(y4Pos-borderSize)))
				if consolidated {
					sub.frame.redCandleSegments = append(sub.frame.redCandleSegments, bor1)
					sub.frame.redCandleSegments = append(sub.frame.redCandleSegments, bor2)
				} else {
					sub.frame.unsureRedCandleSegments = append(sub.frame.unsureRedCandleSegments, bor1)
					sub.frame.unsureRedCandleSegments = append(sub.frame.unsureRedCandleSegments, bor2)
				}
			}
		}
	}
}

func (sub *SubPlot) getValueRange(maxDiff float64) float64 {
	newValueRange := math.Pow10(stockval.CountDigits(int64(math.Ceil(maxDiff))))
	decimalPlaces := sub.maxDecimalPlaces
	if decimalPlaces > 0 {
		// use one less decimal place for the total range than what is supported for each value.
		decimalPlaces--
	}
	decimalBase := math.Pow10(decimalPlaces)
	if maxDiff <= stockval.NearZero {
		maxDiff = 1 / decimalBase
	}
	// Repeatedly divide value range by 2 as long as it is larger than max value
	// Only consider a specified amount of decimal places to avoid strange ranges.
	for {
		nextRange := math.Floor((newValueRange/2)*decimalBase) / decimalBase
		if maxDiff <= nextRange {
			newValueRange = nextRange
		} else {
			break
		}
		// There is also a O(1) version of this but it would be much slower for the usual cases.
	}
	return newValueRange
}

func (sub *SubPlot) resetVolumeSegments() {
	sub.frame.greenVolumeSegments = sub.frame.greenVolumeSegments[:0]
	sub.frame.redVolumeSegments = sub.frame.redVolumeSegments[:0]
	sub.frame.unsureGreenVolumeSegments = sub.frame.unsureGreenVolumeSegments[:0]
	sub.frame.unsureRedVolumeSegments = sub.frame.unsureRedVolumeSegments[:0]
}

func (sub *SubPlot) plotVolumeBars(data *stockval.CandlePlotData, gtx layout.Context) {
	sub.resetVolumeSegments()
	// Only draw within the plot area.
	clipRect := image.Rectangle{Min: sub.frame.minPos, Max: sub.frame.maxPos}
	defer clip.Rect(clipRect).Push(gtx.Ops).Pop()

	proj := sub.frame.projection
	// Base position is always the same, the bar is painted starting at the X axis.
	y2Pos := sub.frame.projection.getYpos(0)

	var maxYvalue float64
	data.DataMutex.RLock()
	for _, d := range data.Data {
		v, _ := d.Volume.Float64()
		o, _ := d.OpenPrice.Float64()
		c, _ := d.ClosePrice.Float64()
		sub.plotSingleBar(v, d.Timestamp, data.Resolution, proj, y2Pos, &maxYvalue, stockval.IsGreenCandle(o, c), true, clipRect, gtx)
	}
	data.DataMutex.RUnlock()
	data.RealtimeData.DataMutex.RLock()
	for i := range data.RealtimeData.Data {
		v, _ := data.RealtimeData.Data[i].Volume.Float64()
		var o, c float64
		if data.HasValidRealtimePrices(i) {
			o, _ = data.RealtimeData.Data[i].OpenPrice.Float64()
			c, _ = data.RealtimeData.Data[i].ClosePrice.Float64()
		}
		sub.plotSingleBar(v, data.RealtimeData.Data[i].Timestamp, data.Resolution, proj, y2Pos, &maxYvalue, stockval.IsGreenCandle(o, c), data.RealtimeData.OpenConsolidated[i], clipRect, gtx)
	}
	data.RealtimeData.DataMutex.RUnlock()
	barWidth, _, _ := getCandleWidth(sub.frame.projection.mX, gtx.Dp(1))
	sub.strokeCandleSegments(gtx, sub.frame.greenVolumeSegments, float32(barWidth), sub.Theme.BarUpColor)
	sub.strokeCandleSegments(gtx, sub.frame.redVolumeSegments, float32(barWidth), sub.Theme.BarDownColor)
	sub.strokeCandleSegments(gtx, sub.frame.unsureGreenVolumeSegments, float32(barWidth), sub.Theme.BarUnknownColor)
	sub.strokeCandleSegments(gtx, sub.frame.unsureRedVolumeSegments, float32(barWidth), sub.Theme.BarUnknownColor)

	sub.autoZoomGenericY(maxYvalue, gtx)
}

func (sub *SubPlot) autoZoomGenericY(maxYvalue float64, gtx layout.Context) {
	maxPlottableYvalue := sub.calcYvalueRange()
	// Auto-Zoom subplot to better fit data.
	if maxYvalue > stockval.NearZero && (maxYvalue > maxPlottableYvalue || maxYvalue < maxPlottableYvalue/2) {
		// Use rounded value range because this is generic data.
		sub.nextValueRangeY = sub.getValueRange(maxYvalue)
		// Redraw this subplot with new value range settings.
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (sub *SubPlot) plotSingleBar(v float64, t time.Time, r candles.CandleResolution, proj projection, y2Pos float64,
	maxV *float64, isGreen bool, consolidated bool, clipRect image.Rectangle, gtx layout.Context) {
	barWidth, _, _ := getCandleWidth(proj.mX, gtx.Dp(1))
	xPos := sub.frame.projection.getXpos(t, r)
	y1Pos := sub.frame.projection.getYpos(v)
	if math.Round(y1Pos) == math.Round(y2Pos) {
		y2Pos++ // Use a minimum height of 1 px
	}

	// Performance: Skip entries where both positions are outside of the visible plot range
	// (both on the same side of the plot only).
	// This is a simplified version for volume bars which considers X only.
	// This will also implicitly be done by clipping to the plot area,
	// but we are filtering here to avoid paint operations in case there is a lot of data.
	if !(int(xPos)+barWidth/2 < clipRect.Min.X || int(xPos)-barWidth/2 > clipRect.Max.X) {
		// Draw a single bar.
		// Use Y-Position minus one, so that we do not draw onto the X-axes, but above it.
		bar1 := stroke.MoveTo(f32.Pt(float32(xPos), float32(y1Pos-1)))
		bar2 := stroke.LineTo(f32.Pt(float32(xPos), float32(y2Pos-1)))

		if isGreen {
			if consolidated {
				sub.frame.greenVolumeSegments = append(sub.frame.greenVolumeSegments, bar1)
				sub.frame.greenVolumeSegments = append(sub.frame.greenVolumeSegments, bar2)
			} else {
				sub.frame.unsureGreenVolumeSegments = append(sub.frame.unsureGreenVolumeSegments, bar1)
				sub.frame.unsureGreenVolumeSegments = append(sub.frame.unsureGreenVolumeSegments, bar2)
			}
		} else {
			if consolidated {
				sub.frame.redVolumeSegments = append(sub.frame.redVolumeSegments, bar1)
				sub.frame.redVolumeSegments = append(sub.frame.redVolumeSegments, bar2)
			} else {
				sub.frame.unsureRedVolumeSegments = append(sub.frame.unsureRedVolumeSegments, bar1)
				sub.frame.unsureRedVolumeSegments = append(sub.frame.unsureRedVolumeSegments, bar2)
			}
		}
		if v > *maxV {
			*maxV = v
		}
	}
}

func (sub *SubPlot) plotQuoteLine(quote stockval.QuoteData, gtx layout.Context, th *material.Theme) {
	if quote.CurrentPrice == nil || quote.PreviousClosePrice == nil || quote.DeltaPercentage == nil {
		return
	}
	clipRect := image.Rectangle{Min: sub.frame.minPos, Max: sub.frame.maxPos}
	minPlotAreaX := clipRect.Min.X
	maxPlotAreaX := clipRect.Max.X
	clipRect.Max.X = sub.frame.totalPxSize.X // we also paint axes text
	defer clip.Rect(clipRect).Push(gtx.Ops).Pop()

	// Plot a dotted line.
	p, _ := quote.CurrentPrice.Float64()
	// yPos may not be scaled yet, but that's fine because https://github.com/andybalholm/stroke/issues/7 was fixed.
	yPos := sub.frame.projection.getYpos(p)

	var quotePath stroke.Path
	quotePath.Segments = []stroke.Segment{
		stroke.MoveTo(f32.Pt(float32(maxPlotAreaX), float32(yPos))),
		stroke.LineTo(f32.Pt(float32(minPlotAreaX), float32(yPos))),
	}
	paint.FillShape(
		gtx.Ops,
		sub.Theme.QuoteDashColor,
		stroke.Stroke{Path: quotePath, Width: float32(gtx.Dp(1)), Dashes: stroke.Dashes{Dashes: sub.Theme.QuoteDashPattern}}.Op(gtx.Ops),
	)
	// Plot current price on thick background line
	var bgColor color.NRGBA
	if stockval.IsGreenQuote(quote.DeltaPercentage) {
		bgColor = sub.Theme.QuoteUpColor
	} else {
		bgColor = sub.Theme.QuoteDownColor
	}
	labelText := sub.formatYlabel(p)
	// Record drawing to pre-calculate text size.
	call, textSize := recordAxesLabelText(labelText, sub.Theme.QuoteTextColor, sub.Theme.AxesYfontSize, gtx, th)

	borderSize := gtx.Dp(2)
	basePosX := sub.frame.yAxesTextPosX
	var markPath stroke.Path
	markPath.Segments = []stroke.Segment{
		stroke.MoveTo(f32.Pt(float32(basePosX), float32(yPos))),
		stroke.LineTo(f32.Pt(float32(basePosX+textSize.X), float32(yPos))),
	}
	paint.FillShape(
		gtx.Ops,
		bgColor,
		stroke.Stroke{Path: markPath, Width: float32(textSize.Y + borderSize), Cap: stroke.RoundCap}.Op(gtx.Ops),
	)

	textArea := op.Offset(image.Point{X: basePosX, Y: int(yPos) - textSize.Y/2}).Push(gtx.Ops)
	// Run recorded drawing.
	call.Add(gtx.Ops)
	textArea.Pop()

	if !sub.hasInitialCandleY && !sub.hasInitialQuoteY {
		// Initially scroll to quote price if no other data is available.
		sub.nextBaseValueY = p
		sub.hasInitialQuoteY = true
		gtx.Execute(op.InvalidateCmd{})
	}
}
