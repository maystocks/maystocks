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

func (q *QuoteField) amount() (*decimal.Big, bool) {
	return new(decimal.Big).SetString(q.amountField.Text())
}

func (q *QuoteField) BuyClicked() (*decimal.Big, bool) {
	if q.buttonBuy.Clicked() {
		return q.amount()
	} else {
		return nil, false
	}
}

func (q *QuoteField) Layout(gtx layout.Context, th *material.Theme, pth *PlotTheme, entry stockval.AssetData, quote stockval.QuoteData,
	bidAsk stockval.RealtimeBidAskData) layout.Dimensions {
	var tradeFieldDims layout.Dimensions

	amount, ok := q.amount()
	hasAmount := ok && stockval.IsGreaterThanZero(amount)
	canSell := false
	canBuy := false

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
						if stockval.IsGreaterThanZero(bidAsk.BidPrice) {
							canSell = true
							sellText = fmt.Sprintf("Sell\n%f\n%d", stockval.PrepareFormattedPrice(bidAsk.BidPrice), bidAsk.BidSize)
						} else {
							sellText = "Sell\n--\n--"
						}
						sellButton := material.Button(th, &q.buttonSell, sellText)
						if !hasAmount || !canSell {
							gtx = gtx.Disabled()
						}
						return sellButton.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints = gtx.Constraints.AddMin(image.Point{X: gtx.Dp(100)})
						gtx.Constraints.Max.X = gtx.Constraints.Min.X
						return q.amountField.Layout(gtx, th, "Amount")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						var buyText string
						if stockval.IsGreaterThanZero(bidAsk.AskPrice) {
							canBuy = true
							buyText = fmt.Sprintf("Buy\n%f\n%d", stockval.PrepareFormattedPrice(bidAsk.AskPrice), bidAsk.AskSize)
						} else {
							buyText = "Buy\n--\n--"
						}
						buyButton := material.Button(th, &q.buttonBuy, buyText)
						if !hasAmount || !canBuy {
							gtx = gtx.Disabled()
						}
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
