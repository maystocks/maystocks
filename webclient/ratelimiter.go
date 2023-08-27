// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package webclient

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// Simple bucket rate limiter (client side) which optionally considers http headers.
// Sadly, I could not find a proper client library for this job.
type RateLimiter struct {
	limitCounter uint64 // Use atomic accessor
	interval     int64  // Use atomic accessor
	startTime    int64  // Use atomic accessor
}

const MinWaitTime = time.Millisecond * 250
const MinReconnectWaitTime = time.Second * 10

// Create a rate limiter to be initialized by http headers.
// Call HandleResponseHeaders to initialize.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{}
}

// Manually initialize rate limiter. Call HandleResponseHeader to initialize start time.
func NewManualRateLimiter(interval time.Duration, limit uint32) *RateLimiter {
	return &RateLimiter{
		limitCounter: uint64(limit) << 32,
		interval:     int64(interval),
	}
}

func (l *RateLimiter) Wait(ctx context.Context) error {
	for {
		limitCounter := atomic.LoadUint64(&l.limitCounter)
		limit := limitCounter >> 32
		if limit == 0 {
			return nil // no limitation
		}
		counter := limitCounter & 0xffffffff

		interval := atomic.LoadInt64(&l.interval)
		startTime := atomic.LoadInt64(&l.startTime)
		if interval > 0 && startTime > 0 {
			endTime := time.UnixMilli(startTime).Add(time.Duration(interval))
			// reset counter after time interval
			if time.Since(endTime) > 0 {
				if atomic.CompareAndSwapInt64(&l.startTime, startTime, endTime.UnixMilli()) {
					// Subtract instead of setting to zero in order to avoid race conditions.
					atomic.AddUint64(&l.limitCounter, -counter)
					limitCounter -= counter
					counter = 0
				} else {
					continue
				}
			}
		}
		if counter < limit {
			if atomic.CompareAndSwapUint64(&l.limitCounter, limitCounter, limitCounter+1) {
				return nil
			} else {
				continue
			}
		}
		// too many requests, need to wait
		// poll every MinWaitTime
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(MinWaitTime):
		}
	}
}

// Return the remaining count or max int if not limited.
func (l *RateLimiter) Remaining() int {
	limitCounter := atomic.LoadUint64(&l.limitCounter)
	limit := limitCounter >> 32
	if limit == 0 {
		return math.MaxInt
	}
	counter := limitCounter & 0xffffffff
	remaining := int(limit) - int(counter)
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}

// In order to make sure the counter is initialized 100% correct, a non-parallel
// first call to HandleResponseHeaders needs to be done.
// However, if the retry return value is properly handled, this does not need to be 100% correct.
// It's fine then to run this Handler in parallel.
func (l *RateLimiter) HandleResponseHeadersWithWait(ctx context.Context, resp *http.Response) (retry bool, err error) {
	if resp.StatusCode == 429 {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(MinWaitTime): // enforce some delay if the server complains
			return true, nil
		}
	}
	if atomic.LoadUint64(&l.limitCounter) == 0 {
		limit, err := strconv.ParseInt(resp.Header.Get("x-ratelimit-limit"), 10, 32)
		if err != nil {
			limit, err = strconv.ParseInt(resp.Header.Get("ratelimit-limit"), 10, 32)
		}
		if err == nil && limit > 0 {
			interval := time.Minute // default rate limit reset interval
			// use custom interval from header if provided.
			resetUnixTime, err := strconv.ParseInt(resp.Header.Get("x-ratelimit-reset"), 10, 64)
			if err == nil && resetUnixTime > 0 {
				resetTime := time.Unix(resetUnixTime, 0)
				timeDiff := time.Until(resetTime).Round(time.Second * 10)
				if timeDiff > 0 {
					interval = timeDiff
				}
			} else {
				resetRemainingSeconds, err := strconv.ParseInt(resp.Header.Get("ratelimit-reset"), 10, 32)
				if err == nil && resetRemainingSeconds > 0 {
					interval = time.Second * time.Duration(resetRemainingSeconds)
				}
			}
			// Set limit and remember this was the first call, so count is already 1.
			if atomic.CompareAndSwapUint64(&l.limitCounter, 0, (uint64(limit)<<32)|1) {
				atomic.CompareAndSwapInt64(&l.startTime, 0, time.Now().UnixMilli())
				atomic.StoreInt64(&l.interval, int64(interval))
			} else {
				atomic.AddUint64(&l.limitCounter, 1) // We might break the limit here, so use a non-parallel first call.
			}
		}
	} else {
		l.HandleManualTimer()
	}
	return false, nil
}

func (l *RateLimiter) HandleManualTimer() {
	if atomic.LoadInt64(&l.interval) > 0 && atomic.LoadInt64(&l.startTime) == 0 {
		// Initialize start time after first call.
		atomic.CompareAndSwapInt64(&l.startTime, 0, time.Now().UnixMilli())
	}
}
