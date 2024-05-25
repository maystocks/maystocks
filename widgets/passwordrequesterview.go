// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type PasswordRequesterView struct {
	passwordList       widget.List
	focusUpdated       bool
	confirmed          bool
	resetRequested     bool
	buttonContinue     widget.Clickable
	buttonReset        widget.Clickable
	buttonConfirmReset widget.Clickable
	buttonCancelReset  widget.Clickable
	passwordTextField  component.TextField
	resetTextField     component.TextField
	note               string
	Margin             unit.Dp
	confirmedPassword  string
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
		resetTextField: component.TextField{
			Editor: widget.Editor{SingleLine: true, MaxLen: 16},
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
		v.confirmedPassword = v.passwordTextField.Text()
		v.confirmed = true
	}
}

func (v *PasswordRequesterView) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if !v.focusUpdated {
		gtx.Execute(key.FocusCmd{Tag: &v.passwordTextField.Editor})
		v.focusUpdated = true
	}
	if v.buttonContinue.Clicked(gtx) {
		v.submitPassword()
	}
	if v.buttonReset.Clicked(gtx) {
		v.resetRequested = true
		gtx.Execute(key.FocusCmd{Tag: &v.resetTextField.Editor})
	}
	if v.buttonCancelReset.Clicked(gtx) {
		v.resetRequested = false
		gtx.Execute(key.FocusCmd{Tag: &v.passwordTextField.Editor})
	}
	if v.buttonConfirmReset.Clicked(gtx) {
		if strings.EqualFold(v.resetTextField.Text(), "reset") {
			v.confirmedPassword = ""
			v.confirmed = true
		} else {
			gtx.Execute(key.FocusCmd{Tag: &v.resetTextField.Editor})
		}
	}
	for {
		evt, ok := v.passwordTextField.Editor.Update(gtx)
		if !ok {
			break
		}
		switch evt.(type) {
		case widget.ChangeEvent:
			v.note = ""
		case widget.SubmitEvent:
			v.submitPassword()
		}
	}

	return layoutConfirmationFrame(th, v.Margin, gtx, &v.buttonContinue, nil, nil, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &v.passwordList).Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(heading(th, "maystocks Startup").Layout),
				layout.Rigid(subHeading(th, "Enter password to decrypt the configuration data.").Layout),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutLabelTextField(
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
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutLabelWidget(
						th,
						v.Margin,
						gtx,
						"Forgot your password?",
						func(gtx layout.Context) layout.Dimensions {
							if v.resetRequested {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layoutTextFieldWithNote(th, gtx, &v.resetTextField, "reset", "Enter 'reset' to confirm resetting configuration data", true)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Left: v.Margin}.Layout(gtx, material.Button(th, &v.buttonConfirmReset, "Confirm reset").Layout)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Left: v.Margin}.Layout(gtx, material.Button(th, &v.buttonCancelReset, "Cancel").Layout)
									}),
								)
							} else {
								return layout.Flex{}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return material.Button(th, &v.buttonReset, "Reset configuration data").Layout(gtx)
									}))
							}
						},
					)
				},
				),
			)
		},
		)
	})
}

func (v *PasswordRequesterView) SetErrorNote(n string) {
	v.note = n
	v.confirmed = false
	v.confirmedPassword = ""
	v.focusUpdated = false
	v.passwordTextField.SetCaret(0, len(v.passwordTextField.Text()))
}

func (v *PasswordRequesterView) validate() bool {
	if len(v.passwordTextField.Text()) < 6 {
		v.SetErrorNote("The minimum password length is 6 characters.")
		return false
	}
	return true
}
