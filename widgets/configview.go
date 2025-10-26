// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"maystocks/config"
	"maystocks/stockval"
	"sort"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/maps"
)

type BrokerView struct {
	BrokerId stockval.BrokerId
	config.BrokerConfig
	note               string
	highlightNote      bool
	apiKeyTextField    component.TextField
	apiSecretTextField component.TextField
	registrationLink   LinkButton
}

type ConfigView struct {
	configList      widget.List
	plotCountEnum   widget.Enum
	buttonContinue  widget.Clickable
	buttonClose     widget.Clickable
	confirmed       bool
	Margin          unit.Dp
	paHash          string
	configChildren  []layout.FlexChild
	brokerConfig    []BrokerView
	numPlots        []image.Point
	paButton        LinkButton
	changePwButton  widget.Clickable
	encryptionSetup config.EncryptionSetup
	pwCreatorView   *PasswordCreatorView
	forceSave       bool
}

const (
	patreonUrl = "https://www.patreon.com/maystocks"
)

func NewConfigView(defaultBrokerConfig map[stockval.BrokerId]config.BrokerConfig, encryptionSetup config.EncryptionSetup) *ConfigView {
	v := ConfigView{
		configList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		Margin:          DefaultMargin,
		brokerConfig:    make([]BrokerView, len(defaultBrokerConfig)),
		numPlots:        make([]image.Point, 1),
		encryptionSetup: encryptionSetup,
	}
	v.paButton.SetUrl(patreonUrl, "Patreon")
	brokerIds := stockval.BrokerList(maps.Keys(defaultBrokerConfig))
	sort.Sort(brokerIds)
	for i, id := range brokerIds {
		v.brokerConfig[i].BrokerId = id
		v.brokerConfig[i].BrokerConfig = defaultBrokerConfig[id]
		v.brokerConfig[i].apiSecretTextField.Mask = '·'
		if v.brokerConfig[i].OptionalKey {
			v.brokerConfig[i].note = "optional but recommended"
		} else {
			v.brokerConfig[i].note = "at least one broker needs to be configured"
		}
		v.brokerConfig[i].apiKeyTextField.SingleLine = true
		v.brokerConfig[i].apiSecretTextField.SingleLine = true
	}
	return &v
}

// Call from same goroutine as Layout
func (v *ConfigView) GetWindowConfig(appConfig *config.AppConfig) {
	appConfig.WindowConfig[0].NumPlots = v.numPlots[0]
	appConfig.Sanitize() // create default plot configurations if needed
}

// Call from same goroutine as Layout
func (v *ConfigView) SetWindowConfig(appConfig *config.AppConfig) {
	v.numPlots[0] = appConfig.WindowConfig[0].NumPlots
	// TODO use dynamic window count
	v.plotCountEnum.Value = v.numPlots[0].String()
}

// Call from same goroutine as Layout
func (v *ConfigView) UpdateConfigFromUi(appConfig *config.AppConfig) bool {
	for i := range v.brokerConfig {
		c := appConfig.BrokerConfig[v.brokerConfig[i].BrokerId]
		c.ApiKey = v.brokerConfig[i].ApiKey
		c.ApiSecret = v.brokerConfig[i].ApiSecret
		appConfig.BrokerConfig[v.brokerConfig[i].BrokerId] = c
	}
	return v.forceSave
}

// Call from same goroutine as Layout
func (v *ConfigView) UpdateUiFromConfig(appConfig *config.AppConfig) {
	for i := range v.brokerConfig {
		c, exists := appConfig.BrokerConfig[v.brokerConfig[i].BrokerId]
		if exists {
			v.brokerConfig[i].BrokerConfig = c
			v.brokerConfig[i].apiKeyTextField.SetText(v.brokerConfig[i].ApiKey)
			v.brokerConfig[i].apiSecretTextField.SetText(v.brokerConfig[i].ApiSecret)
			v.brokerConfig[i].registrationLink.SetUrl(v.brokerConfig[i].RegistrationUrl, "")
		}
	}
}

// Call from same goroutine as Layout
func (v *ConfigView) ConfirmClicked() bool {
	c := v.confirmed
	v.confirmed = false
	return c
}

func (v *ConfigView) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if v.buttonContinue.Clicked(gtx) || v.buttonClose.Clicked(gtx) {
		if v.validate() {
			for i := range v.brokerConfig {
				v.brokerConfig[i].ApiKey = v.brokerConfig[i].apiKeyTextField.Text()
				v.brokerConfig[i].ApiSecret = v.brokerConfig[i].apiSecretTextField.Text()
			}
			numPlotsStr := strings.Trim(v.plotCountEnum.Value, "()")
			numPlotsSlice := strings.Split(numPlotsStr, ",")
			if len(numPlotsSlice) == 2 {
				v.numPlots[0].X, _ = strconv.Atoi(numPlotsSlice[0])
				v.numPlots[0].Y, _ = strconv.Atoi(numPlotsSlice[1])
			}
			v.confirmed = true
		}
	}
	if v.changePwButton.Clicked(gtx) {
		v.pwCreatorView = NewPasswordCreatorView(true)
	}
	if v.pwCreatorView != nil {
		if v.pwCreatorView.ConfirmClicked() {
			if len(v.pwCreatorView.confirmedExistingPw) > 0 && v.encryptionSetup.IsEncryptionPassword(v.pwCreatorView.confirmedExistingPw) {
				v.encryptionSetup.SetEncryptionPassword(v.pwCreatorView.confirmedNewPw)
				v.forceSave = true
				v.pwCreatorView = nil
			} else {
				v.pwCreatorView.SetErrorNoteCurPw("Current password is not valid")
			}
		} else if v.pwCreatorView.CancelClicked() {
			v.pwCreatorView = nil
		}
		if v.pwCreatorView != nil {
			return v.pwCreatorView.Layout(th, gtx)
		}
	}

	return layoutConfirmationFrame(th, v.Margin, gtx, &v.buttonContinue, nil, &v.buttonClose, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &v.configList).Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
			v.configChildren = v.configChildren[:0]
			v.configChildren = append(v.configChildren,
				layout.Rigid(heading(th, "Plot Settings").Layout),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutLabelWidget(th, v.Margin, gtx, "Number of plots:", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.RadioButton(
									th,
									&v.plotCountEnum,
									"(1,1)",
									"1",
								).Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.RadioButton(
									th,
									&v.plotCountEnum,
									"(2,1)",
									"1 row 2 cols",
								).Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.RadioButton(
									th,
									&v.plotCountEnum,
									"(1,2)",
									"2 rows 1 col",
								).Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.RadioButton(
									th,
									&v.plotCountEnum,
									"(2,2)",
									"2 rows 2 cols",
								).Layout(gtx)
							}),
							// TODO due to api limits, these are not working very well
							/*									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																	return material.RadioButton(
																		th,
																		&v.plotCountEnum,
																		"(2,3)",
																		"2 rows 3 cols",
																	).Layout(gtx)
																}),
																layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																	return material.RadioButton(
																		th,
																		&v.plotCountEnum,
																		"(3,3)",
																		"3 rows 3 cols",
																	).Layout(gtx)
																}),*/
						)
					})
				}),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(heading(th, "Support this Project").Layout),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutLabelWidget(th, v.Margin, gtx, "Your donations help to fund further development!", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if len(v.paHash) == 0 {
									hash := sha256.Sum256([]byte(v.paButton.Url()))
									v.paHash = hex.EncodeToString(hash[:])
								}
								if v.paHash == "82863cd5e5cda4aa73a465912ecbe0e64e29b8c322e7d2998fe530b08afc7c51" {
									return v.paButton.Layout(th, gtx)
								} else {
									return layout.Dimensions{}
								}
							}))
					})
				}),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(heading(th, "Broker Settings").Layout),
				layout.Rigid(subHeading(th, "(changes require restart)").Layout),
			)
			for i := range v.brokerConfig {
				v.configChildren = v.appendBrokerLayout(th, &v.brokerConfig[i], v.configChildren)
			}
			v.configChildren = append(v.configChildren,
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(heading(th, "Secure configuration data").Layout),
				layout.Rigid(divider(th, v.Margin).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutLabelWidget(th, v.Margin, gtx, "Change encryption password", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Button(th, &v.changePwButton, "Change password").Layout(gtx)
							}))
					})
				}),
			)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, v.configChildren...)
		},
		)
	})
}

func (v *ConfigView) validate() bool {
	if v.plotCountEnum.Value == "" {
		return false
	}
	var hasValidBroker bool
	for i := range v.brokerConfig {
		if !v.brokerConfig[i].OptionalKey {
			if v.brokerConfig[i].IsValid() {
				hasValidBroker = true
				break
			} else {
				v.brokerConfig[i].highlightNote = true
			}
		}
	}
	return hasValidBroker
}

func (v *ConfigView) appendBrokerLayout(th *material.Theme, b *BrokerView, children []layout.FlexChild) []layout.FlexChild {
	children = append(children, layout.Rigid(divider(th, v.Margin).Layout))
	children = append(children,
		v.linkChild(th, &b.registrationLink, ""))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layoutLabelTextField(th, v.Margin, gtx, &b.apiKeyTextField, string(b.BrokerId)+" API key:", string(b.BrokerId)+" key", b.note, b.highlightNote)
	}))
	if b.UseApiSecret {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutLabelTextField(th, v.Margin, gtx, &b.apiSecretTextField, string(b.BrokerId)+" API secret:", string(b.BrokerId)+" secret", "", false)
		}))
	}
	return children
}

func (v *ConfigView) linkChild(th *material.Theme, link *LinkButton, label string) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layoutLabelWidget(th, 0, gtx, label, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return link.Layout(th, gtx)
				}))
		})
	})
}

func (b *BrokerView) IsValid() bool {
	if len(b.apiKeyTextField.Text()) == 0 {
		return false
	}
	if b.UseApiSecret && len(b.apiSecretTextField.Text()) == 0 {
		return false
	}
	return true
}
