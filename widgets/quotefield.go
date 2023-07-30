// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"fmt"
	"image"
	"maystocks/stockval"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/ericlagergren/decimal"
)

type QuoteField struct {
	amountField component.TextField
	buttonBuy   widget.Clickable
	buttonSell  widget.Clickable
}

func NewQuoteField() *QuoteField {
	return &QuoteField{
		amountField: component.TextField{
			Editor: widget.Editor{Submit: true, SingleLine: true, MaxLen: 32},
		},
	}
}

func (q *QuoteField) SellClicked() (*decimal.Big, bool) {
	if q.buttonSell.Clicked() {
		amount, ok := new(decimal.Big).SetString(q.amountField.Text())
		return amount, ok
	} else {
		return nil, false
	}
}

func (q *QuoteField) BuyClicked() (*decimal.Big, bool) {
	if q.buttonBuy.Clicked() {
		amount, ok := new(decimal.Big).SetString(q.amountField.Text())
		return amount, ok
	} else {
		return nil, false
	}
}

func (q *QuoteField) Layout(gtx layout.Context, th *material.Theme, pth *PlotTheme, entry stockval.AssetData, quote stockval.QuoteData,
	bidAsk stockval.RealtimeBidAskData) layout.Dimensions {
	var tradeFieldDims layout.Dimensions

	return Frame{InnerMargin: 5, BorderWidth: 1, BorderColor: pth.FrameBgColor, BackgroundColor: pth.FrameBgColor}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:    layout.Vertical,
			Spacing: layout.SpaceEnd,
		}.Layout(
			gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !entry.Tradable {
					return layout.Dimensions{}
				}
				tradeFieldDims = layout.Flex{}.Layout(
					gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						var sellText string
						if bidAsk.BidPrice != nil && bidAsk.BidPrice.CmpTotal(new(decimal.Big)) > 0 {
							sellText = fmt.Sprintf("Sell\n%f\n%d", stockval.PrepareFormattedPrice(bidAsk.BidPrice), bidAsk.BidSize)
						} else {
							sellText = "Sell\n--\n--"
						}
						sellButton := material.Button(th, &q.buttonSell, sellText)
						return sellButton.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints = gtx.Constraints.AddMin(image.Point{X: gtx.Dp(100)})
						gtx.Constraints.Max.X = gtx.Constraints.Min.X
						return q.amountField.Layout(gtx, th, "Amount")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						var buyText string
						if bidAsk.AskPrice != nil && bidAsk.AskPrice.CmpTotal(new(decimal.Big)) > 0 {
							buyText = fmt.Sprintf("Buy\n%f\n%d", stockval.PrepareFormattedPrice(bidAsk.AskPrice), bidAsk.AskSize)
						} else {
							buyText = "Buy\n--\n--"
						}
						buyButton := material.Button(th, &q.buttonBuy, buyText)
						return buyButton.Layout(gtx)
					}),
				)
				return tradeFieldDims
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				var quoteText string
				if quote.CurrentPrice != nil && quote.DeltaPercentage != nil {
					percentage := stockval.RoundPercentage(quote.DeltaPercentage)
					var prefix string
					if percentage.Sign() >= 0 {
						prefix = "+"
					}
					quoteText = fmt.Sprintf("%f (%s%f%%)", stockval.PrepareFormattedPrice(quote.CurrentPrice), prefix, percentage)
				} else {
					quoteText = "-- (-- %)"
				}
				lblQuote := material.Body1(
					th,
					quoteText,
				)
				lblQuote.Color = pth.FrameTextColor
				lblQuote.Alignment = text.Middle
				gtx.Constraints.Min.X = tradeFieldDims.Size.X
				return lblQuote.Layout(gtx)
			}),
		)
	})
}

/*	var callQuote op.CallOp
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
	var dimsBid, dimsAsk layout.Dimensions
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
		dimsAsk = lblAsk.Layout(gtx)
		callAsk = macroAsk.Stop()
	}

	clipRect := image.Rectangle{Max: image.Point{X: gtx.Dp(100), Y: gtx.Dp(70) + dimsQuote.Size.Y + dimsBid.Size.Y}}
	defer clip.Rect(clipRect).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, color.NRGBA{R: 50, G: 50, B: 50, A: 200})

	textAreaQuote := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(25), Y: clipRect.Min.Y + gtx.Dp(10)}).Push(gtx.Ops)
	callQuote.Add(gtx.Ops)
	textAreaQuote.Pop()

	textAreaBid := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(25), Y: clipRect.Min.Y + gtx.Dp(10) + dimsQuote.Size.Y}).Push(gtx.Ops)
	callBid.Add(gtx.Ops)
	textAreaBid.Pop()

	textAreaAsk := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(50) + dimsBid.Size.X, Y: clipRect.Min.Y + gtx.Dp(10) + dimsQuote.Size.Y}).Push(gtx.Ops)
	callAsk.Add(gtx.Ops)
	textAreaAsk.Pop()

	if entry.Tradable {
		textAreaAmount := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(75) + dimsBid.Size.X, Y: clipRect.Min.Y + gtx.Dp(10) + dimsQuote.Size.Y}).Push(gtx.Ops)
		q.amountField.Layout(gtx, th, "Amount")
		textAreaAmount.Pop()
	}
	return layout.Dimensions{Size: clipRect.Max}
}*/
