// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockplot

import (
	"maystocks/stockval"
	"maystocks/widgets"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget/material"
)

func LayoutTitleField(gtx layout.Context, th *material.Theme, pth *widgets.PlotTheme, entry stockval.AssetData) layout.Dimensions {
	return widgets.Frame{InnerMargin: 5, BorderWidth: 1, BorderColor: pth.FrameBgColor, BackgroundColor: pth.FrameBgColor}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(
			gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lblName := material.H6(
					th,
					stockval.TruncateDisplayName(entry.CompanyName),
				)
				lblName.Color = pth.FrameTextColor
				lblName.Alignment = text.Start
				return lblName.Layout(gtx)
			}),
		)
	})
}
