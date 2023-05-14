// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
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
	configList     widget.List
	plotCountEnum  widget.Enum
	buttonContinue widget.Clickable
	confirmed      bool
	Margin         unit.Dp
	ppHash         string
	paHash         string
	configChildren []layout.FlexChild
	brokerConfig   []BrokerView
	numPlots       []image.Point
	ppButton       LinkButton
	paButton       LinkButton
}

const (
	paypalUrl  = "https://www.paypal.com/donate/?hosted_button_id=RJVXEZH6LEWQN"
	patreonUrl = "https://www.patreon.com/maystocks"
)

func NewConfigView(defaultBrokerConfig map[stockval.BrokerId]config.BrokerConfig) *ConfigView {
	v := ConfigView{
		configList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		Margin:       DefaultMargin,
		brokerConfig: make([]BrokerView, len(defaultBrokerConfig)),
		numPlots:     make([]image.Point, 1),
	}
	v.ppButton.SetUrl(paypalUrl, "PayPal")
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
	}
	return &v
}

func (v *ConfigView) GetWindowConfig(appConfig *config.AppConfig) {
	appConfig.WindowConfig[0].NumPlots = v.numPlots[0]
	appConfig.Sanitize() // create default plot configurations if needed
}

func (v *ConfigView) SetWindowConfig(appConfig *config.AppConfig) {
	v.numPlots[0] = appConfig.WindowConfig[0].NumPlots
	// TODO use dynamic window count
	v.plotCountEnum.Value = v.numPlots[0].String()
}

func (v *ConfigView) GetBrokerConfig(appConfig *config.AppConfig) {
	for i := range v.brokerConfig {
		c := appConfig.BrokerConfig[v.brokerConfig[i].BrokerId]
		c.ApiKey = v.brokerConfig[i].ApiKey
		c.ApiSecret = v.brokerConfig[i].ApiSecret
		appConfig.BrokerConfig[v.brokerConfig[i].BrokerId] = c
	}
}

func (v *ConfigView) SetBrokerConfig(appConfig *config.AppConfig) {
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
	if v.buttonContinue.Clicked() {
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

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return Frame{InnerMargin: v.Margin / 2, OuterMargin: v.Margin, BorderWidth: 1, BorderColor: th.Palette.ContrastBg, BackgroundColor: th.Palette.Bg}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.List(th, &v.configList).Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
					v.configChildren = v.configChildren[:0]
					v.configChildren = append(v.configChildren,
						layout.Rigid(heading(th, "Plot Settings").Layout),
						layout.Rigid(divider(th, v.Margin).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return v.layoutConfigEntry(th, gtx, "Number of plots:", v.Margin, func(gtx layout.Context) layout.Dimensions {
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
							return v.layoutConfigEntry(th, gtx, "Your donations help to fund further development!", v.Margin, func(gtx layout.Context) layout.Dimensions {
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
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return v.layoutConfigEntry(th, gtx, "", v.Margin, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if len(v.ppHash) == 0 {
											hash := sha256.Sum256([]byte(v.ppButton.Url()))
											v.ppHash = hex.EncodeToString(hash[:])
										}
										if v.ppHash == "78f09dff6b492d6eff4d1ab3a7f25f7830b88d4ddc5e2cc3b61cec3743f03667" {
											return v.ppButton.Layout(th, gtx)
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
						v.configChildren = v.appendBrokerLayout(th, gtx, &v.brokerConfig[i], v.configChildren)
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, v.configChildren...)
				},
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: v.Margin, Bottom: v.Margin, Left: v.Margin}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Button(th, &v.buttonContinue, "Done").Layout(gtx)
					}),
				)
			})
		},
		),
	)
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

func (v *ConfigView) layoutConfigEntry(th *material.Theme, gtx layout.Context, label string, margin unit.Dp, w layout.Widget) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: margin * 2, Bottom: margin}.Layout(gtx, material.Body1(th, label).Layout)
				}))
		}),
		layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: margin}.Layout(gtx, w)
		},
		),
	)
}

func (v *ConfigView) appendBrokerLayout(th *material.Theme, gtx layout.Context, b *BrokerView, children []layout.FlexChild) []layout.FlexChild {
	children = append(children, layout.Rigid(divider(th, v.Margin).Layout))
	children = append(children,
		v.linkChild(th, &b.registrationLink, ""))
	children = append(children,
		v.textConfigChild(th, &b.apiKeyTextField, &b.registrationLink, string(b.BrokerId)+" API key:", string(b.BrokerId)+" key", b.note, b.highlightNote))
	if b.UseApiSecret {
		children = append(children,
			v.textConfigChild(th, &b.apiSecretTextField, nil, string(b.BrokerId)+" API secret:", string(b.BrokerId)+" secret", "", false))
	}
	return children
}

func (v *ConfigView) textConfigChild(th *material.Theme, field *component.TextField, link *LinkButton, label string, hint string, note string, highlightNote bool) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return v.layoutConfigEntry(th, gtx, label, v.Margin, func(gtx layout.Context) layout.Dimensions {
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
		})
	})
}

func (v *ConfigView) linkChild(th *material.Theme, link *LinkButton, label string) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return v.layoutConfigEntry(th, gtx, label, 0, func(gtx layout.Context) layout.Dimensions {
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
