// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

import (
	"maystocks/stockval"

	"github.com/barkimedes/go-deepcopy"
)

type AppConfig struct {
	IsEncrypted      bool `yaml:"-"`
	LicenseConfirmed bool `yaml:",omitempty"`
	LightTheme       bool `yaml:",omitempty"`
	BrokerConfig     map[stockval.BrokerId]BrokerConfig
	WindowConfig     []WindowConfig
}

type BrokerConfig struct {
	DataUrl         string `yaml:",omitempty"`
	TradingUrl      string `yaml:",omitempty"`
	PaperTradingUrl string `yaml:",omitempty"`
	AppTradingUrl   string `yaml:",omitempty"`
	RegistrationUrl string `yaml:",omitempty"`
	WsUrl           string `yaml:",omitempty"`
	ApiKey          string `yaml:",omitempty"`
	ApiSecret       string `yaml:",omitempty"`
	UseApiSecret    bool   `yaml:",omitempty"`
	OptionalKey     bool   `yaml:",omitempty"`
	// According to https://finnhub.io/docs/api/rate-limit there is a general rate limit per second
	RateLimitPerSecond int `yaml:",omitempty"`
	// e.g. finnhub sometimes does not reply, so use a timeout.
	DataTimeoutSeconds int `yaml:",omitempty"`
}

var defaultBrokerConfig = NewBrokerConfigMap()

func NewAppConfig() AppConfig {
	return AppConfig{
		BrokerConfig: NewBrokerConfigMap(),
		WindowConfig: []WindowConfig{NewWindowConfig()},
	}
}

func NewBrokerConfigMap() map[stockval.BrokerId]BrokerConfig {
	return map[stockval.BrokerId]BrokerConfig{
		"finnhub": {
			DataUrl:            "https://finnhub.io/api/v1",
			RegistrationUrl:    "https://finnhub.io/",
			WsUrl:              "wss://ws.finnhub.io",
			RateLimitPerSecond: 30,
			DataTimeoutSeconds: 10,
		},
		"alpaca": {
			DataUrl:            "https://data.alpaca.markets/v2",
			TradingUrl:         "https://api.alpaca.markets/v2",
			PaperTradingUrl:    "https://paper-api.alpaca.markets/v2",
			AppTradingUrl:      "https://app.alpaca.markets/trade/%s",
			RegistrationUrl:    "https://alpaca.markets/",
			WsUrl:              "wss://stream.data.alpaca.markets/v2",
			UseApiSecret:       true,
			DataTimeoutSeconds: 10,
		},
		"openfigi": {
			DataUrl:            "https://api.openfigi.com/v3",
			RegistrationUrl:    "https://www.openfigi.com/",
			OptionalKey:        true,
			DataTimeoutSeconds: 10,
		},
	}
}

func (a *AppConfig) deepCopy() AppConfig {
	c, err := deepcopy.Anything(a)
	if err != nil {
		panic(err)
	}
	return *c.(*AppConfig)
}

func (a *AppConfig) Sanitize() {
	if len(a.WindowConfig) == 0 {
		a.WindowConfig = append(a.WindowConfig, NewWindowConfig())
	}
	for i := range a.WindowConfig {
		a.WindowConfig[i].sanitize()
	}
	a.RestoreDefaults()
}

// We do not want to store certain default values in the configuration file,
// in order to avoid having to patch them.
func (a *AppConfig) RemoveDefaults() {
	for key, c := range a.BrokerConfig {
		def := defaultBrokerConfig[key]
		if c.DataUrl == def.DataUrl {
			c.DataUrl = ""
		}
		if c.PaperTradingUrl == def.PaperTradingUrl {
			c.PaperTradingUrl = ""
		}
		if c.RegistrationUrl == def.RegistrationUrl {
			c.RegistrationUrl = ""
		}
		if c.TradingUrl == def.TradingUrl {
			c.TradingUrl = ""
		}
		if c.WsUrl == def.WsUrl {
			c.WsUrl = ""
		}
		a.BrokerConfig[key] = c
	}
}

// Restore certain default values which are not stored in the configuration file.
func (a *AppConfig) RestoreDefaults() {
	for key, c := range a.BrokerConfig {
		def := defaultBrokerConfig[key]
		if len(c.DataUrl) == 0 {
			c.DataUrl = def.DataUrl
		}
		if len(c.PaperTradingUrl) == 0 {
			c.PaperTradingUrl = def.PaperTradingUrl
		}
		if len(c.RegistrationUrl) == 0 {
			c.RegistrationUrl = def.RegistrationUrl
		}
		if len(c.TradingUrl) == 0 {
			c.TradingUrl = def.TradingUrl
		}
		if len(c.WsUrl) == 0 {
			c.WsUrl = def.WsUrl
		}
		if len(c.AppTradingUrl) == 0 {
			c.AppTradingUrl = def.AppTradingUrl
		}
		a.BrokerConfig[key] = c
	}
}
