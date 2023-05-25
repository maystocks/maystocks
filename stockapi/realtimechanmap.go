// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockapi

import (
	"fmt"
	"maystocks/stockval"
	"sync"

	"github.com/zhangyunhao116/skipmap"
)

type RealtimeChanMap[T any] struct {
	sm                    *skipmap.StringMap[chan T]
	pendingCloseList      []chan T
	pendingCloseListMutex *sync.Mutex
}

func NewRealtimeChanMap[T any]() *RealtimeChanMap[T] {
	return &RealtimeChanMap[T]{
		sm:                    skipmap.NewString[chan T](),
		pendingCloseListMutex: new(sync.Mutex),
	}
}

func (m *RealtimeChanMap[T]) AddPendingClose(c chan T) {
	m.pendingCloseListMutex.Lock()
	m.pendingCloseList = append(m.pendingCloseList, c)
	m.pendingCloseListMutex.Unlock()
}

func (m *RealtimeChanMap[T]) ClearPendingClose() {
	m.pendingCloseListMutex.Lock()
	for _, c := range m.pendingCloseList {
		close(c)
	}
	m.pendingCloseList = nil
	m.pendingCloseListMutex.Unlock()
}

func (m *RealtimeChanMap[T]) Clear() {
	m.sm.Range(
		func(k string, c chan T) bool {
			close(c)
			return true
		},
	)
	m.sm = skipmap.NewString[chan T]()
}

func (m *RealtimeChanMap[T]) Subscribe(entry stockval.AssetData) (chan T, error) {
	// this is required to be a buffered channel, so that it is possible to delete old data in case processing is too slow
	// new realtime data is always more important than old data
	// TODO size of chan
	c := make(chan T, 1024)
	var err error
	_, exists := m.sm.LoadOrStore(entry.Symbol, c)
	if exists {
		err = fmt.Errorf("already subscribed to %s", entry.Symbol)
		c = nil
	}
	return c, err
}

func (m *RealtimeChanMap[T]) Unsubscribe(entry stockval.AssetData) error {
	var err error
	if c, exists := m.sm.LoadAndDelete(entry.Symbol); exists {
		// we should not close the channel here, because this might cause a race condition.
		m.AddPendingClose(c)
	} else {
		err = fmt.Errorf("cannot unsubscribe %s: not subscribed", entry.Symbol)
	}
	return err
}

func (m *RealtimeChanMap[T]) AddNewData(symbol string, data T) error {
	c, exists := m.sm.Load(symbol)
	var err error
	if exists {
		select {
		case c <- data:
		// usually if a golang channel is full, we would drop additional data.
		// but new data is much more important in this case, so instead we
		// delete old data.
		// we might steal one entry without necessity in some corner cases,
		// but in general this code is fine.
		default:
			select {
			// try to remove first entry, non-blocking
			case <-c:
				// try again to push the new entry, non-blocking
				select {
				case c <- data:
					err = fmt.Errorf("Symbol %s: Buffer overflow. Old realtime data is being removed.", symbol)
				default:
					err = fmt.Errorf("Symbol %s: Buffer overflow. New realtime data is being dropped.", symbol)
				}
			default:
				err = fmt.Errorf("Symbol %s: Buffer cannot be read from or written to.", symbol)
			}
		}
	}
	// silently ignore if entry does not exist, as this may happen while unsubscribing
	return err
}
