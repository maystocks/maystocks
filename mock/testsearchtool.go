// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package mock

import (
	"context"
	"errors"
	"maystocks/config"
	"maystocks/stockapi"
)

type TestSearchTool struct {
}

func NewSearchTool() stockapi.SymbolSearchTool {
	return &TestSearchTool{}
}

func (ts *TestSearchTool) GetCapabilities() stockapi.Capabilities {
	return stockapi.Capabilities{}
}

func (ts *TestSearchTool) RemainingApiLimit() int {
	return 1
}

func (ts *TestSearchTool) ReadConfig(c config.Config) error {
	return nil
}

func (ts *TestSearchTool) FindAsset(ctx context.Context, entry <-chan stockapi.SearchRequest, response chan<- stockapi.SearchResponse) {
	defer close(response)

	for entry := range entry {
		response <- stockapi.SearchResponse{SearchRequest: entry, Error: errors.New("no test data")}
	}
}
