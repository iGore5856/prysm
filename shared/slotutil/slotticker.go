// Package slotutil includes ticker and timer-related functions for eth2.
package slotutil

import (
	"time"

	types "github.com/farazdagi/prysm-shared-types"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
)

// The Ticker interface defines a type which can expose a
// receive-only channel firing slot events.
type Ticker interface {
	C() <-chan types.Slot
	Done()
}

// SlotTicker is a special ticker for the beacon chain block.
// The channel emits over the slot interval, and ensures that
// the ticks are in line with the genesis time. This means that
// the duration between the ticks and the genesis time are always a
// multiple of the slot duration.
// In addition, the channel returns the new slot number.
type SlotTicker struct {
	c    chan types.Slot
	done chan struct{}
}

// C returns the ticker channel. Call Cancel afterwards to ensure
// that the goroutine exits cleanly.
func (s *SlotTicker) C() <-chan types.Slot {
	return s.c
}

// Done should be called to clean up the ticker.
func (s *SlotTicker) Done() {
	go func() {
		s.done <- struct{}{}
	}()
}

// GetSlotTicker is the constructor for SlotTicker.
func GetSlotTicker(genesisTime time.Time, secondsPerSlot uint64) *SlotTicker {
	if genesisTime.Unix() == 0 {
		panic("zero genesis time")
	}
	ticker := &SlotTicker{
		c:    make(chan types.Slot),
		done: make(chan struct{}),
	}
	ticker.start(genesisTime, secondsPerSlot, timeutils.Since, timeutils.Until, time.After)
	return ticker
}

// GetSlotTickerWithOffset is a constructor for SlotTicker that allows a offset of time from genesis,
// entering a offset greater than secondsPerSlot is not allowed.
func GetSlotTickerWithOffset(genesisTime time.Time, offset time.Duration, secondsPerSlot uint64) *SlotTicker {
	if genesisTime.Unix() == 0 {
		panic("zero genesis time")
	}
	if offset > time.Duration(secondsPerSlot)*time.Second {
		panic("invalid ticker offset")
	}
	ticker := &SlotTicker{
		c:    make(chan types.Slot),
		done: make(chan struct{}),
	}
	ticker.start(genesisTime.Add(offset), secondsPerSlot, timeutils.Since, timeutils.Until, time.After)
	return ticker
}

func (s *SlotTicker) start(
	genesisTime time.Time,
	secondsPerSlot uint64,
	since, until func(time.Time) time.Duration,
	after func(time.Duration) <-chan time.Time) {

	d := time.Duration(secondsPerSlot) * time.Second

	go func() {
		sinceGenesis := since(genesisTime)

		var nextTickTime time.Time
		var slot types.Slot
		if sinceGenesis < d {
			// Handle when the current time is before the genesis time.
			nextTickTime = genesisTime
			slot = 0
		} else {
			nextTick := sinceGenesis.Truncate(d) + d
			nextTickTime = genesisTime.Add(nextTick)
			slot = types.Slot(nextTick / d)
		}

		for {
			waitTime := until(nextTickTime)
			select {
			case <-after(waitTime):
				s.c <- slot
				slot++
				nextTickTime = nextTickTime.Add(d)
			case <-s.done:
				return
			}
		}
	}()
}
