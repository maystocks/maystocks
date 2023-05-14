// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"
)

type MessageField struct {
}

func NewMessageField() *MessageField {
	return &MessageField{}
}

func (f *MessageField) Layout(txt string, gtx layout.Context, th *material.Theme) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	lbl := material.Body1(th, txt)
	dims := lbl.Layout(gtx)
	call := macro.Stop()

	clipRect := image.Rectangle{Max: image.Point{X: gtx.Dp(50) + dims.Size.X, Y: gtx.Dp(40) + dims.Size.Y}}
	defer clip.Rect(clipRect).Push(gtx.Ops).Pop()
	// TODO use theme color or property
	paint.Fill(gtx.Ops, color.NRGBA{R: 150, G: 0, B: 0, A: 250})

	textArea := op.Offset(image.Point{X: clipRect.Min.X + gtx.Dp(25), Y: clipRect.Min.Y + gtx.Dp(30) - dims.Size.Y/2}).Push(gtx.Ops)
	// Run recorded drawing.
	call.Add(gtx.Ops)
	textArea.Pop()
	return layout.Dimensions{Size: clipRect.Size()}
}
