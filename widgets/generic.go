// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
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
