// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package mock

import (
	"context"
	"maystocks/cache"
	"maystocks/stockval"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestAssetCache struct {
	t *testing.T
}

func NewAssetCache(t *testing.T) cache.AssetCache {
	return &TestAssetCache{t: t}
}

func (c *TestAssetCache) GetAssetList(ctx context.Context, req func(ctx context.Context) ([]stockval.AssetData, error)) cache.AssetList {
	d, err := req(ctx)
	assert.Nil(c.t, err)
	return d
}
