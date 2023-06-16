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
	passwordList        widget.List
	focusUpdated        bool
	confirmed           bool
	cancelled           bool
	requestExisting     bool
	buttonContinue      widget.Clickable
	buttonCancel        widget.Clickable
	existingPwTextField component.TextField
	newPwTextField      component.TextField
	newPw2ndTextField   component.TextField
	noteNewPw           string
	noteCurPw           string
	Margin              unit.Dp
	confirmedNewPw      string
	confirmedExistingPw string
}

func NewPasswordCreatorView(requestExisting bool) *PasswordCreatorView {
	v := PasswordCreatorView{
		passwordList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		requestExisting: requestExisting,
		existingPwTextField: component.TextField{
			Editor: widget.Editor{Submit: true, SingleLine: true, MaxLen: 128, Mask: '·'},
		},
		newPwTextField: component.TextField{
			Editor: widget.Editor{Submit: true, SingleLine: true, MaxLen: 128, Mask: '·'},
		},
		newPw2ndTextField: component.TextField{
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
func (v *PasswordCreatorView) CancelClicked() bool {
	c := v.cancelled
	v.cancelled = false
	return c
}

// Call from same goroutine as Layout
func (v *PasswordCreatorView) GetConfirmedPassword() string {
	return v.confirmedNewPw
}

func (v *PasswordCreatorView) GetExistingPassword() string {
	return v.confirmedExistingPw
}

func (v *PasswordCreatorView) submitPassword() {
	if v.validate() {
		v.confirmed = true
		v.confirmedExistingPw = v.existingPwTextField.Text()
		v.confirmedNewPw = v.newPwTextField.Text()
	}
}

func (v *PasswordCreatorView) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if !v.focusUpdated {
		if v.requestExisting {
			v.existingPwTextField.Focus()
		} else {
			v.newPwTextField.Focus()
		}
		v.focusUpdated = true
	}
	if v.buttonContinue.Clicked() {
		v.submitPassword()
	}
	if v.buttonCancel.Clicked() {
		v.cancelled = true
	}
	for _, evt := range v.existingPwTextField.Events() {
		switch evt.(type) {
		case widget.ChangeEvent:
			v.noteNewPw = ""
		case widget.SubmitEvent:
			v.newPwTextField.Focus()
		}
	}
	for _, evt := range v.newPwTextField.Events() {
		switch evt.(type) {
		case widget.ChangeEvent:
			v.noteNewPw = ""
		case widget.SubmitEvent:
			v.newPw2ndTextField.Focus()
		}
	}
	for _, evt := range v.newPw2ndTextField.Events() {
		switch evt.(type) {
		case widget.ChangeEvent:
			v.noteNewPw = ""
		case widget.SubmitEvent:
			v.submitPassword()
		}
	}

	var buttonCancel *widget.Clickable
	if v.requestExisting {
		buttonCancel = &v.buttonCancel
	}
	return layoutConfirmationFrame(th, v.Margin, gtx, &v.buttonContinue, buttonCancel, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &v.passwordList).Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(heading(th, "Secure configuration data").Layout),
				layout.Rigid(subHeading(th, "The configuration data will be stored locally and encrypted using a password.").Layout),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if v.requestExisting {
						return layoutLabelTextField(
							th,
							v.Margin,
							gtx,
							&v.existingPwTextField,
							"Enter existing password:",
							"Configuration data password",
							v.noteCurPw,
							true,
						)
					} else {
						return layout.Dimensions{}
					}
				},
				),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutLabelTextField(
						th,
						v.Margin,
						gtx,
						&v.newPwTextField,
						"Enter new password:",
						"Configuration data password",
						v.noteNewPw,
						true,
					)
				},
				),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutLabelTextField(
						th,
						v.Margin,
						gtx,
						&v.newPw2ndTextField,
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

func (v *PasswordCreatorView) SetErrorNoteCurPw(n string) {
	v.noteCurPw = n
	v.confirmed = false
	v.confirmedExistingPw = ""
	v.confirmedNewPw = ""
	v.focusUpdated = false
	v.existingPwTextField.SetCaret(0, len(v.existingPwTextField.Text()))
}

func (v *PasswordCreatorView) SetErrorNoteNewPw(n string) {
	v.noteNewPw = n
	v.confirmed = false
	v.confirmedExistingPw = ""
	v.confirmedNewPw = ""
	v.focusUpdated = false
	v.newPwTextField.SetCaret(0, len(v.newPwTextField.Text()))
}

func (v *PasswordCreatorView) validate() bool {
	if len(v.newPwTextField.Text()) < 6 {
		v.SetErrorNoteNewPw("The minimum password length is 6 characters.")
		return false
	}
	if v.newPwTextField.Text() != v.newPw2ndTextField.Text() {
		v.SetErrorNoteNewPw("Passwords do not match.")
		return false
	}
	return true
}
