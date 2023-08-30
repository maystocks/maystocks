// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package cache

import (
	"context"
	"encoding/json"
	"log"
	"maystocks/config"
	"maystocks/stockval"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/lotodore/localcache"
)

const CacheKeyStockSymbols = "stocksymbols"

type localAssetCache struct {
	broker   stockval.BrokerId
	data     *localcache.Cache
	initLock sync.Mutex
}

func NewLocalAssetCache(broker stockval.BrokerId) AssetCache {
	c := localAssetCache{
		broker: broker,
	}
	var err error
	c.data, err = localcache.New(filepath.Join(config.AppName, string(broker)))
	if err != nil {
		log.Fatalf("error initializing asset cache: %v", err)
	}
	return &c
}

func (c *localAssetCache) GetAssetList(ctx context.Context, req func(ctx context.Context) ([]stockval.AssetData, error)) AssetList {
	// Cache stock symbols for some hours.
	err := c.data.PurgeKey(CacheKeyStockSymbols, time.Hour*12)
	if err != nil {
		log.Printf("error purging cache %s, symbol data may be outdated", CacheKeyStockSymbols)
	}
	symbols := c.readSymbolsFromCache()
	if symbols == nil {
		symbols, err = c.initSymbolCache(ctx, req)
		if err != nil {
			log.Printf("error requesting stock symbols: %v", err)
		}
	}
	if symbols == nil {
		log.Printf("error loading %s stock symbols, lookup is not available", c.broker)
		symbols = make([]stockval.AssetData, 0)
	}
	return symbols
}

func (c *localAssetCache) readSymbolsFromCache() []stockval.AssetData {
	rawSymbols, err := c.data.ReadFile(CacheKeyStockSymbols)
	if err == nil {
		var symbols []stockval.AssetData
		err := json.Unmarshal(rawSymbols, &symbols)
		if err == nil {
			return symbols
		}
		log.Printf("%s symbol cache contains invalid data", c.broker)
		err = c.data.Remove(CacheKeyStockSymbols)
		if err != nil {
			log.Printf("error deleting cache %s, symbol data may be invalid", CacheKeyStockSymbols)
		}
	}
	return nil
}

func (c *localAssetCache) initSymbolCache(ctx context.Context, req func(ctx context.Context) ([]stockval.AssetData, error)) ([]stockval.AssetData, error) {
	c.initLock.Lock()
	defer c.initLock.Unlock()
	// retry reading cache within lock, to avoid requesting the data twice.
	cachedSymbols := c.readSymbolsFromCache()
	if cachedSymbols != nil {
		return cachedSymbols, nil
	}
	log.Printf("requesting %s stock symbols...", c.broker)
	symbols, err := req(ctx)
	if err != nil {
		return nil, err
	}
	sort.Sort(AssetList(symbols))
	symbolsText, err := json.Marshal(&symbols)
	if err != nil {
		return nil, err
	}
	err = c.data.WriteFile(CacheKeyStockSymbols, symbolsText)
	if err != nil {
		return nil, err
	}
	return symbols, nil
}
