// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"fmt"
	"maystocks/calendar"
	"maystocks/stockval"
	"time"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget/material"
)

type QuoteField struct {
	calendar    calendar.BankCalendar
	buttonTrade *LinkButton
}

func NewQuoteField(brokerName string, tradingAppUrl string) *QuoteField {

	q := QuoteField{
		calendar: calendar.NewUSBankCalendar(),
	}
	if len(tradingAppUrl) > 0 {
		q.buttonTrade = &LinkButton{}
		q.buttonTrade.SetUrl(tradingAppUrl, "Trade on "+brokerName)
	}
	return &q
}

func (q *QuoteField) Layout(gtx layout.Context, th *material.Theme, pth *PlotTheme, entry stockval.AssetData, quote stockval.QuoteData,
	bidAsk stockval.RealtimeBidAskData) layout.Dimensions {
	var tradeFieldDims layout.Dimensions
	var quoteLabelDims layout.Dimensions

	return Frame{InnerMargin: 5, BorderWidth: 1, BorderColor: pth.FrameBgColor, BackgroundColor: pth.FrameBgColor}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Vertical,
			Spacing:   layout.SpaceEnd,
			Alignment: layout.Middle,
		}.Layout(
			gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !entry.Tradable || q.buttonTrade == nil {
					return layout.Dimensions{}
				}
				tradeFieldDims = layout.Flex{
					Axis:      layout.Vertical,
					Spacing:   layout.SpaceEnd,
					Alignment: layout.Middle,
				}.Layout(
					gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(
							gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								var sellText string
								if stockval.IsGreaterThanZero(bidAsk.BidPrice) {
									sellText = fmt.Sprintf("Bid\n%f\n%d", stockval.PrepareFormattedPrice(bidAsk.BidPrice), bidAsk.BidSize)
								} else {
									sellText = "Bid\n--\n--"
								}
								lblBid := material.Body1(
									th,
									sellText,
								)
								lblBid.Color = pth.FrameTextColor
								lblBid.Alignment = text.Middle
								return layout.Inset{Right: 10, Left: 10}.Layout(gtx, lblBid.Layout)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								var buyText string
								if stockval.IsGreaterThanZero(bidAsk.AskPrice) {
									buyText = fmt.Sprintf("Ask\n%f\n%d", stockval.PrepareFormattedPrice(bidAsk.AskPrice), bidAsk.AskSize)
								} else {
									buyText = "Ask\n--\n--"
								}
								lblAsk := material.Body1(
									th,
									buyText,
								)
								lblAsk.Color = pth.FrameTextColor
								lblAsk.Alignment = text.Middle
								return layout.Inset{Right: 10, Left: 10}.Layout(gtx, lblAsk.Layout)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: 5, Left: 5, Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return q.buttonTrade.Layout(th, gtx)
						})
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
				quoteLabelDims = lblQuote.Layout(gtx)
				return quoteLabelDims
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				var hintText string
				tradingTime := time.Now()
				isHoliday, holidayName := q.calendar.IsBankHoliday(tradingTime)
				if isHoliday {
					hintText = holidayName
				} else {
					trading, _, h := q.calendar.GetTradingHours(tradingTime)
					if trading {
						hintText = h.GetTradingState(tradingTime)
					} else {
						hintText = "Weekend (no trading)"
					}
				}
				if len(hintText) == 0 {
					return layout.Dimensions{}
				}
				lblHint := material.Body1(
					th,
					hintText,
				)
				lblHint.Color = pth.FrameTextColor
				lblHint.Alignment = text.Middle
				gtx.Constraints.Min.X = quoteLabelDims.Size.X
				return lblHint.Layout(gtx)
			}),
		)
	})
}
