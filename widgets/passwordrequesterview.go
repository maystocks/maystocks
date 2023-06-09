// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type PasswordRequesterView struct {
	passwordList      widget.List
	focusUpdated      bool
	confirmed         bool
	buttonContinue    widget.Clickable
	passwordTextField component.TextField
	note              string
	Margin            unit.Dp
	confirmedPassword string
}

func NewPasswordRequesterView() *PasswordRequesterView {
	v := PasswordRequesterView{
		passwordList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		passwordTextField: component.TextField{
			Editor: widget.Editor{Submit: true, SingleLine: true, MaxLen: 128, Mask: '·'},
		},
		Margin: DefaultMargin,
	}
	return &v
}

// Call from same goroutine as Layout
func (v *PasswordRequesterView) ConfirmClicked() bool {
	c := v.confirmed
	v.confirmed = false
	return c
}

// Call from same goroutine as Layout
func (v *PasswordRequesterView) GetConfirmedPassword() string {
	return v.confirmedPassword
}

func (v *PasswordRequesterView) submitPassword() {
	if v.validate() {
		v.confirmed = true
		v.confirmedPassword = v.passwordTextField.Text()
	}
}

func (v *PasswordRequesterView) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if !v.focusUpdated {
		v.passwordTextField.Focus()
		v.focusUpdated = true
	}
	if v.buttonContinue.Clicked() {
		v.submitPassword()
	}
	for _, evt := range v.passwordTextField.Events() {
		switch evt.(type) {
		case widget.ChangeEvent:
			v.note = ""
		case widget.SubmitEvent:
			v.submitPassword()
		}
	}

	return layoutConfirmationFrame(th, v.Margin, gtx, &v.buttonContinue, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &v.passwordList).Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(heading(th, "maystocks Startup").Layout),
				layout.Rigid(subHeading(th, "Enter password to decrypt configuration data.").Layout),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutConfigChild(
						th,
						v.Margin,
						gtx,
						&v.passwordTextField,
						"Password:",
						"Configuration data password",
						v.note,
						true,
					)
				},
				),
			)
		},
		)
	})
}

func (v *PasswordRequesterView) validate() bool {
	if len(v.passwordTextField.Text()) < 6 {
		v.note = "The minimum password length is 6 characters."
		return false
	}
	return true
}
