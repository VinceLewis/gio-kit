package guitest

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"

	"gioui.org/io/transfer"
)

var (
	ErrClipboardUnavailable = errors.New("guitest: clipboard unavailable")
	ErrClipboardDenied      = errors.New("guitest: clipboard denied")
	ErrClipboardTooLarge    = errors.New("guitest: clipboard data too large")
)

// ClipboardAdapter replaces the platform clipboard behind Gio's real
// clipboard.ReadCmd and clipboard.WriteCmd routing. Methods run on worker
// goroutines and must honor context cancellation.
type ClipboardAdapter interface {
	ReadClipboard(context.Context) (string, error)
	WriteClipboard(context.Context, string, []byte) error
}

type clipboardRead struct {
	generation uint64
	text       string
	err        error
}

type clipboardBridge struct {
	mu              sync.Mutex
	ctx             context.Context
	adapter         ClipboardAdapter
	maxBytes        int
	invalidate      func()
	wg              sync.WaitGroup
	readCancel      context.CancelFunc
	writeCancel     context.CancelFunc
	readGeneration  uint64
	writeGeneration uint64
	read            *clipboardRead
	writeErr        error
	writeReady      bool
	closed          bool
}

func newClipboardBridge(ctx context.Context, adapter ClipboardAdapter, maxBytes int, invalidate func()) *clipboardBridge {
	return &clipboardBridge{ctx: ctx, adapter: adapter, maxBytes: maxBytes, invalidate: invalidate}
}

func (c *clipboardBridge) startRead() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if c.readCancel != nil {
		c.readCancel()
	}
	c.readGeneration++
	generation := c.readGeneration
	ctx, cancel := context.WithCancel(c.ctx)
	c.readCancel = cancel
	adapter := c.adapter
	c.wg.Add(1)
	c.mu.Unlock()

	go func() {
		defer c.wg.Done()
		text, err := "", ErrClipboardUnavailable
		if adapter != nil {
			text, err = adapter.ReadClipboard(ctx)
		}
		if err == nil && len(text) > c.maxBytes {
			text, err = "", ErrClipboardTooLarge
		}
		c.mu.Lock()
		if c.closed || generation != c.readGeneration {
			c.mu.Unlock()
			return
		}
		c.read = &clipboardRead{generation: generation, text: text, err: err}
		invalidate := c.invalidate
		c.mu.Unlock()
		if invalidate != nil {
			invalidate()
		}
	}()
}

func (c *clipboardBridge) takeRead() (clipboardRead, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.read == nil || c.read.generation != c.readGeneration {
		return clipboardRead{}, false
	}
	result := *c.read
	c.read = nil
	return result, true
}

func (c *clipboardBridge) startWrite(mime string, data []byte) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if c.writeCancel != nil {
		c.writeCancel()
	}
	c.writeGeneration++
	generation := c.writeGeneration
	ctx, cancel := context.WithCancel(c.ctx)
	c.writeCancel = cancel
	adapter := c.adapter
	maxBytes := c.maxBytes
	content := append([]byte(nil), data...)
	c.wg.Add(1)
	c.mu.Unlock()

	go func() {
		defer c.wg.Done()
		var err error
		switch {
		case adapter == nil:
			err = ErrClipboardUnavailable
		case len(content) > maxBytes:
			err = ErrClipboardTooLarge
		default:
			err = adapter.WriteClipboard(ctx, mime, content)
		}
		c.mu.Lock()
		if c.closed || generation != c.writeGeneration {
			c.mu.Unlock()
			return
		}
		c.writeErr, c.writeReady = err, true
		invalidate := c.invalidate
		c.mu.Unlock()
		if invalidate != nil {
			invalidate()
		}
	}()
}

func (c *clipboardBridge) takeWriteError() (error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.writeReady {
		return nil, false
	}
	err := c.writeErr
	c.writeErr, c.writeReady = nil, false
	return err, true
}

func (c *clipboardBridge) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	if c.readCancel != nil {
		c.readCancel()
	}
	if c.writeCancel != nil {
		c.writeCancel()
	}
	c.mu.Unlock()
	c.wg.Wait()
	return nil
}

func (d *Driver) receiveClipboard() {
	if d.clipboard == nil {
		return
	}
	if err, ok := d.clipboard.takeWriteError(); ok {
		d.clipboardErr = err
	}
	result, ok := d.clipboard.takeRead()
	if !ok {
		return
	}
	d.clipboardErr = result.err
	text := result.text
	if result.err != nil {
		text = ""
	}
	d.router.Queue(transfer.DataEvent{
		Type: "application/text",
		Open: func() io.ReadCloser { return io.NopCloser(strings.NewReader(text)) },
	})
}

func (d *Driver) sendClipboard() {
	if d.clipboard == nil {
		return
	}
	if d.router.ClipboardRequested() {
		d.clipboardErr = nil
		d.clipboard.startRead()
	}
	if mime, content, ok := d.router.WriteClipboard(); ok {
		d.clipboardErr = nil
		d.clipboard.startWrite(mime, content)
	}
}

// ClipboardError reports the latest completed platform-adapter error. A nil
// result includes successful empty reads and successful writes.
func (d *Driver) ClipboardError() error { return d.clipboardErr }
