// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

const DefaultMargin = 10

type Frame struct {
	OuterMargin     unit.Dp
	InnerMargin     unit.Dp
	BorderWidth     unit.Dp
	BorderColor     color.NRGBA
	BackgroundColor color.NRGBA
}

func (f Frame) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return layout.UniformInset(f.OuterMargin).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Border{Color: f.BorderColor, Width: f.BorderWidth, CornerRadius: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if empty := (color.NRGBA{}); f.BackgroundColor != empty {
				macro := op.Record(gtx.Ops)
				dims := layout.UniformInset(f.InnerMargin).Layout(gtx, w)
				call := macro.Stop()
				paint.FillShape(gtx.Ops, f.BackgroundColor, clip.Rect{Max: dims.Size}.Op())
				call.Add(gtx.Ops)
				return dims
			} else {
				return layout.UniformInset(f.InnerMargin).Layout(gtx, w)
			}
		})
	})
}
