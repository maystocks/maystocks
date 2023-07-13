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
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/ericlagergren/decimal"
)

type QuoteField struct {
	amountField      component.TextField
	buttonBuyMarket  widget.Clickable
	buttonSellMarket widget.Clickable
}

func NewQuoteField() *QuoteField {
	return &QuoteField{}
}

func (q *QuoteField) Layout(entry stockval.AssetData, quote stockval.QuoteData, bidAsk stockval.RealtimeBidAskData, basePos image.Point, gtx layout.Context, th *material.Theme) {
	macroName := op.Record(gtx.Ops)
	lblName := material.H6(
		th,
		stockval.TruncateDisplayName(entry.CompanyName),
	)
	lblName.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	lblName.Alignment = text.Start
	dimsName := lblName.Layout(gtx)
	callName := macroName.Stop()

	var callQuote op.CallOp
	var dimsQuote layout.Dimensions
	if quote.CurrentPrice != nil && quote.DeltaPercentage != nil {
		percentage := stockval.RoundPercentage(quote.DeltaPercentage)
		var prefix string
		if percentage.Sign() >= 0 {
			prefix = "+"
		}
		lblQuote := material.Body1(
			th,
			fmt.Sprintf("%f (%s%f%%)", stockval.PrepareFormattedPrice(quote.CurrentPrice), prefix, percentage),
		)
		lblQuote.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		lblQuote.Alignment = text.Start
		macroQuote := op.Record(gtx.Ops)
		dimsQuote = lblQuote.Layout(gtx)
		callQuote = macroQuote.Stop()
	}

	var callBid, callAsk op.CallOp
	var dimsBid /*, dimsAsk*/ layout.Dimensions
	if bidAsk.AskPrice != nil && bidAsk.AskPrice.CmpTotal(new(decimal.Big)) > 0 &&
		bidAsk.BidPrice != nil && bidAsk.BidPrice.CmpTotal(new(decimal.Big)) > 0 {
		lblBid := material.Body1(
			th,
			fmt.Sprintf("Bid: %f\n%d", stockval.PrepareFormattedPrice(bidAsk.BidPrice), bidAsk.BidSize),
		)
		lblBid.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		lblBid.Alignment = text.Start
		macroBid := op.Record(gtx.Ops)
		dimsBid = lblBid.Layout(gtx)
		callBid = macroBid.Stop()

		lblAsk := material.Body1(
			th,
			fmt.Sprintf("Ask: %f\n%d", stockval.PrepareFormattedPrice(bidAsk.AskPrice), bidAsk.AskSize),
		)
		lblAsk.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		lblAsk.Alignment = text.Start
		macroAsk := op.Record(gtx.Ops)
		/*dimsAsk =*/ lblAsk.Layout(gtx)
		callAsk = macroAsk.Stop()
	}

	clipRect := image.Rectangle{
		Min: basePos.Add(image.Point{X: gtx.Dp(50), Y: gtx.Dp(50)}),
		Max: basePos.Add(image.Point{X: gtx.Dp(100) + dimsName.Size.X, Y: gtx.Dp(70) + dimsName.Size.Y + dimsQuote.Size.Y + dimsBid.Size.Y}),
	}
	defer clip.Rect(clipRect).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, color.NRGBA{R: 50, G: 50, B: 50, A: 200})

	textArea := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(25), Y: clipRect.Min.Y + gtx.Dp(10)}).Push(gtx.Ops)
	callName.Add(gtx.Ops)
	textArea.Pop()

	textAreaQuote := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(25), Y: clipRect.Min.Y + gtx.Dp(10) + dimsName.Size.Y}).Push(gtx.Ops)
	callQuote.Add(gtx.Ops)
	textAreaQuote.Pop()

	textAreaBid := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(25), Y: clipRect.Min.Y + gtx.Dp(10) + dimsName.Size.Y + dimsQuote.Size.Y}).Push(gtx.Ops)
	callBid.Add(gtx.Ops)
	textAreaBid.Pop()

	textAreaAsk := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(50) + dimsBid.Size.X, Y: clipRect.Min.Y + gtx.Dp(10) + dimsName.Size.Y + dimsQuote.Size.Y}).Push(gtx.Ops)
	callAsk.Add(gtx.Ops)
	textAreaAsk.Pop()

	if entry.Tradable {
		q.amountField.Layout(gtx, th, "Amount")
	}
}
