// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package mock

import (
	"bufio"
	"log"
	"maystocks/config"
	"maystocks/stockval"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func NewLogger(t *testing.T) (*log.Logger, *bufio.Scanner) {
	r, w, err := os.Pipe()
	if err != nil {
		assert.Fail(t, "failed to create logger mock: %v", err)
	}
	t.Cleanup(func() { r.Close() })
	t.Cleanup(func() { w.Close() })
	return log.New(w, "", log.LstdFlags), bufio.NewScanner(r)
}

func NewBrokerConfig(brokerId stockval.BrokerId, dataUrl string) config.Config {
	c := NewTestConfig()
	appConfig, _ := c.Lock()
	brokerConfig := appConfig.BrokerConfig[brokerId]
	brokerConfig.DataUrl = dataUrl
	brokerConfig.WsUrl = "ws" + strings.TrimPrefix(dataUrl, "http")
	brokerConfig.PaperTradingUrl = dataUrl
	appConfig.BrokerConfig[brokerId] = brokerConfig
	_ = c.Unlock(appConfig, true)
	return c
}
