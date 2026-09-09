package guitest

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Clock provides virtual frame time and cancellable application delays. Only
// Driver.Advance and due-frame settling advance it; wall time never does.
// Now, Sleep, and Pending are safe to call from asynchronous workers.
type Clock struct {
	mu     sync.Mutex
	now    time.Time
	next   uint64
	timers map[uint64]clockTimer
	notify func()
}

type clockTimer struct {
	at   time.Time
	done chan struct{}
}

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Pending reports registered Sleep calls whose deadlines have not been reached.
func (c *Clock) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

// Sleep waits for virtual time or context cancellation. Tests should observe
// Pending or an application readiness condition before advancing a worker's
// delay: scheduling a goroutine does not mean it has registered its timer yet.
func (c *Clock) Sleep(ctx context.Context, duration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if duration <= 0 {
		return nil
	}
	c.mu.Lock()
	c.next++
	id := c.next
	timer := clockTimer{at: c.now.Add(duration), done: make(chan struct{})}
	c.timers[id] = timer
	c.mu.Unlock()
	c.notify()
	defer func() {
		c.mu.Lock()
		delete(c.timers, id)
		c.mu.Unlock()
		c.notify()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.done:
		return ctx.Err()
	}
}

func (c *Clock) advance(duration time.Duration) error {
	if duration < 0 {
		return errors.New("guitest: cannot move time backwards")
	}
	c.mu.Lock()
	c.now = c.now.Add(duration)
	for id, timer := range c.timers {
		if !timer.at.After(c.now) {
			delete(c.timers, id)
			close(timer.done)
		}
	}
	c.mu.Unlock()
	return nil
}
