// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

func heading(th *material.Theme, t string) material.LabelStyle {
	l := material.H5(th, t)
	l.Alignment = text.Middle
	return l
}

func subHeading(th *material.Theme, t string) material.LabelStyle {
	l := material.Body2(th, t)
	l.Alignment = text.Middle
	return l
}

func divider(th *material.Theme, margin unit.Dp) component.DividerStyle {
	return component.DividerStyle{
		Thickness: unit.Dp(1),
		Fill:      component.WithAlpha(th.ContrastBg, 0x60),
		Inset: layout.Inset{
			Top:    margin,
			Bottom: margin,
		},
	}
}

func layoutLeftRightWidgets(th *material.Theme, margin unit.Dp, gtx layout.Context, left layout.Widget, right layout.Widget) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(0.5, left),
		layout.Flexed(0.5, right),
	)
}

func layoutLabelWidget(th *material.Theme, margin unit.Dp, gtx layout.Context, text string, w layout.Widget) layout.Dimensions {
	return layoutLeftRightWidgets(
		th,
		margin,
		gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: margin * 2, Bottom: margin}.Layout(gtx, material.Body1(th, text).Layout)
				}))
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: margin}.Layout(gtx, w)
		},
	)

}

func layoutConfirmationFrame(th *material.Theme, margin unit.Dp, gtx layout.Context, buttonContinue *widget.Clickable, buttonCancel *widget.Clickable, buttonClose *widget.Clickable, w layout.Widget) layout.Dimensions {
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return Frame{InnerMargin: 0, OuterMargin: margin, BorderWidth: 1, BorderColor: th.Palette.ContrastBg, BackgroundColor: th.Palette.Bg}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{Alignment: layout.NE}.Layout(
					gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(
							gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(margin/2).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return w(gtx)
								})
							}),
						)
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						if buttonClose != nil {
							return layout.Inset{Right: margin + margin/2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return material.Button(th, buttonClose, "X").Layout(gtx)
							})
						} else {
							return layout.Dimensions{}
						}
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if buttonCancel != nil {
				return layout.Inset{Right: margin, Bottom: margin, Left: margin}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return material.Button(th, buttonCancel, "Cancel").Layout(gtx)
						}),
					)
				})
			} else {
				return layout.Dimensions{}
			}
		},
		),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: margin, Bottom: margin, Left: margin}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Button(th, buttonContinue, "Done").Layout(gtx)
					}),
				)
			})
		},
		),
	)
}

func layoutTextFieldWithNote(th *material.Theme, gtx layout.Context, field *component.TextField, hint string, note string, highlightNote bool) layout.Dimensions {
	noteLabel := material.Body2(th, note)
	if highlightNote {
		// TODO use theme
		noteLabel.Color = color.NRGBA{R: 255, A: 255}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return field.Layout(gtx, th, hint)
		}),
		layout.Rigid(noteLabel.Layout),
	)
}

func layoutLabelTextField(th *material.Theme, margin unit.Dp, gtx layout.Context, field *component.TextField, label string, hint string, note string, highlightNote bool) layout.Dimensions {
	return layoutLabelWidget(th, margin, gtx, label, func(gtx layout.Context) layout.Dimensions {
		return layoutTextFieldWithNote(th, gtx, field, hint, note, highlightNote)
	})
}
