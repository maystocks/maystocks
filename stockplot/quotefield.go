// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockplot

import (
	"fmt"
	"image"
	"image/color"
	"maystocks/stockval"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget/material"
)

func LayoutQuoteField(entry stockval.AssetData, quote stockval.QuoteData, basePos image.Point, gtx layout.Context, th *material.Theme) {
	macro := op.Record(gtx.Ops)
	lbl := material.H6(
		th,
		entry.Symbol+"\n"+stockval.TruncateDisplayName(entry.CompanyName),
	)
	lbl.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	lbl.Alignment = text.Start
	dims := lbl.Layout(gtx)
	call := macro.Stop()

	clipRect := image.Rectangle{Min: basePos.Add(image.Point{X: gtx.Dp(50), Y: gtx.Dp(50)}), Max: basePos.Add(image.Point{X: gtx.Dp(100) + dims.Size.X, Y: gtx.Dp(100) + dims.Size.Y})}
	defer clip.Rect(clipRect).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, color.NRGBA{R: 50, G: 50, B: 50, A: 200})

	textArea := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(25), Y: clipRect.Min.Y + gtx.Dp(30) - dims.Size.Y/2}).Push(gtx.Ops)
	// Run recorded drawing.
	call.Add(gtx.Ops)
	textArea.Pop()

	if quote.CurrentPrice != nil && quote.DeltaPercentage != nil {
		macro = op.Record(gtx.Ops)
		percentage := stockval.RoundPercentage(quote.DeltaPercentage)
		var prefix string
		if percentage.Sign() >= 0 {
			prefix = "+"
		}
		lbl = material.Body1(
			th,
			fmt.Sprintf("%f (%s%f%%)", stockval.PrepareFormattedPrice(quote.CurrentPrice), prefix, percentage),
		)
		lbl.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		lbl.Alignment = text.Start
		dims = lbl.Layout(gtx)
		call = macro.Stop()

		textArea = op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(25), Y: clipRect.Min.Y + gtx.Dp(80) - dims.Size.Y/2}).Push(gtx.Ops)
		// Run recorded drawing.
		call.Add(gtx.Ops)
		textArea.Pop()
	}
}
