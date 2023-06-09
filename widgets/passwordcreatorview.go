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

type PasswordCreatorView struct {
	passwordList         widget.List
	focusUpdated         bool
	confirmed            bool
	buttonContinue       widget.Clickable
	passwordTextField    component.TextField
	password2ndTextField component.TextField
	note                 string
	Margin               unit.Dp
	confirmedPassword    string
}

func NewPasswordCreatorView() *PasswordCreatorView {
	v := PasswordCreatorView{
		passwordList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		passwordTextField: component.TextField{
			Editor: widget.Editor{Submit: true, SingleLine: true, MaxLen: 128, Mask: '·'},
		},
		password2ndTextField: component.TextField{
			Editor: widget.Editor{Submit: true, SingleLine: true, MaxLen: 128, Mask: '·'},
		},
		Margin: DefaultMargin,
	}
	return &v
}

// Call from same goroutine as Layout
func (v *PasswordCreatorView) ConfirmClicked() bool {
	c := v.confirmed
	v.confirmed = false
	return c
}

// Call from same goroutine as Layout
func (v *PasswordCreatorView) GetConfirmedPassword() string {
	return v.confirmedPassword
}

func (v *PasswordCreatorView) submitPassword() {
	if v.validate() {
		v.confirmed = true
		v.confirmedPassword = v.passwordTextField.Text()
	}
}

func (v *PasswordCreatorView) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
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
			v.password2ndTextField.Focus()
		}
	}
	for _, evt := range v.password2ndTextField.Events() {
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
				layout.Rigid(heading(th, "Secure configuration data").Layout),
				layout.Rigid(subHeading(th, "The configuration data will be stored locally and encrypted using a password.").Layout),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutConfigChild(
						th,
						v.Margin,
						gtx,
						&v.passwordTextField,
						"Enter new password:",
						"Configuration data password",
						v.note,
						true,
					)
				},
				),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutConfigChild(
						th,
						v.Margin,
						gtx,
						&v.password2ndTextField,
						"Confirm new password:",
						"Configuration data password",
						"",
						false,
					)
				},
				),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(subHeading(th, "Note: Remember this password! You will need to reset the configuration if the password is lost.").Layout),
			)
		},
		)
	})
}

func (v *PasswordCreatorView) validate() bool {
	if len(v.passwordTextField.Text()) < 6 {
		v.note = "The minimum password length is 6 characters."
		return false
	}
	if v.passwordTextField.Text() != v.password2ndTextField.Text() {
		v.note = "Passwords do not match."
		return false
	}
	return true
}
