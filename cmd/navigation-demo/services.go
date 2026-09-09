package main

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VinceLewis/gio-kit/grid"
)

// The application owns this seam; importing gio-kit never imports demo policy.
// Frame time already comes from layout.Context.Now. Artificial I/O delays use
// Sleep so the same root works with a virtual clock in an in-process test.
type demoEnvironment struct {
	Context    context.Context
	DataDir    string
	Invalidate func()
	Sleep      func(context.Context, time.Duration) error
}

func (e demoEnvironment) prepare() (demoEnvironment, context.CancelFunc) {
	if e.Context == nil {
		e.Context = context.Background()
	}
	ctx, cancel := context.WithCancel(e.Context)
	e.Context = ctx
	invalidate := e.Invalidate
	e.Invalidate = func() {
		if ctx.Err() == nil && invalidate != nil {
			invalidate()
		}
	}
	sleep := e.Sleep
	if sleep == nil {
		sleep = func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}
	e.Sleep = func(parent context.Context, delay time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		child, done := context.WithCancel(parent)
		stop := context.AfterFunc(ctx, done)
		defer stop()
		defer done()
		return sleep(child, delay)
	}
	return e, cancel
}

// begin/stop synchronize worker startup against teardown, including a submit
// goroutine that the form controller scheduled but has not started yet.
type taskGroup struct {
	mu      sync.Mutex
	wg      sync.WaitGroup
	closing bool
	count   int
}

func (g *taskGroup) begin() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return false
	}
	g.count++
	g.wg.Add(1)
	return true
}
func (g *taskGroup) end()          { g.mu.Lock(); g.count--; g.mu.Unlock(); g.wg.Done() }
func (g *taskGroup) pending() bool { g.mu.Lock(); defer g.mu.Unlock(); return g.count != 0 }
func (g *taskGroup) stop()         { g.mu.Lock(); g.closing = true; g.mu.Unlock(); g.wg.Wait() }

type stateRequest struct {
	version uint64
	data    []byte
}
type stateResult struct {
	version uint64
	err     error
}
type stateWriter struct {
	requests chan stateRequest
	results  chan stateResult
	done     chan struct{}
	pending  atomic.Int32
	err      error // read only after done is closed
}

func newStateWriter(path string, notify func()) *stateWriter {
	w := &stateWriter{requests: make(chan stateRequest, 1), results: make(chan stateResult, 1), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		for request := range w.requests {
			// A private temporary beside the destination permits atomic replace.
			temporary := path + ".new"
			err := os.WriteFile(temporary, request.data, 0o600)
			if err == nil {
				err = os.Rename(temporary, path)
			}
			w.err = err
			select {
			case <-w.results:
			default:
			}
			w.results <- stateResult{version: request.version, err: err}
			w.pending.Add(-1)
			notify()
		}
	}()
	return w
}

// submit is called on the frame goroutine. Keep the latest queued state while
// a prior write is in progress; neither channel operation blocks layout.
func (w *stateWriter) submit(version uint64, data []byte) {
	w.pending.Add(1)
	select {
	case <-w.requests:
		w.pending.Add(-1)
	default:
	}
	w.requests <- stateRequest{version: version, data: data}
}

func (w *stateWriter) close() error { close(w.requests); <-w.done; return w.err }

func (u *demoUI) Idle() bool {
	if u.externalBusy || u.work.pending() || u.persistence.pending.Load() != 0 || len(u.persistence.results) != 0 {
		return false
	}
	if u.gridDemo.controller == nil && u.gridDemo.setupErr == nil {
		return false
	}
	if c := u.gridDemo.controller; c != nil && c.Snapshot().State == grid.Loading {
		return false
	}
	if u.lookup != nil && u.lookup.ctrl.Snapshot().State == grid.Loading {
		return false
	}
	for _, f := range u.formDemos {
		if f.form.Snapshot().Submitting || len(f.saved) != 0 {
			return false
		}
	}
	return true
}
