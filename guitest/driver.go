package guitest

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"sync/atomic"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"github.com/VinceLewis/gio-kit/diagnostic"
)

var (
	ErrClosed     = errors.New("guitest: driver closed")
	ErrFrameLimit = errors.New("guitest: frame limit exceeded")
)

type LayoutFunc func(layout.Context) layout.Dimensions

// Environment supplies the platform services replaced by an in-process test.
// Applications keep their own storage, theme, records, and application policy.
type Environment struct {
	Context    context.Context
	Clock      *Clock
	Invalidate func()
}

// Harness binds an application's existing root and lifecycle to the driver.
// Idle is evaluated on the frame goroutine and must include queued completions,
// not just running workers. Nil Idle means there is no external work to track.
// Close must cancel and join application-owned workers before releasing storage.
type Harness struct {
	Layout LayoutFunc
	Idle   func() bool
	Close  func() error
	// Providers are queried on the frame goroutine during diagnostic capture.
	Providers map[string]diagnostic.Provider
}

type Option func(*config)

type config struct {
	size                image.Point
	metric              unit.Metric
	locale              system.Locale
	epoch               time.Time
	maxFrames           int
	interval            time.Duration
	bindings            []idBinding
	clipboardConfigured bool
	clipboardAdapter    ClipboardAdapter
	clipboardMaxBytes   int
}

// Size sets physical pixels. Metrics determines the dp/sp conversion.
func Size(width, height int) Option      { return func(c *config) { c.size = image.Pt(width, height) } }
func Metrics(metric unit.Metric) Option  { return func(c *config) { c.metric = metric } }
func Locale(locale system.Locale) Option { return func(c *config) { c.locale = locale } }
func StartTime(epoch time.Time) Option   { return func(c *config) { c.epoch = epoch } }
func MaxFrames(limit int) Option         { return func(c *config) { c.maxFrames = limit } }

// WithClipboard injects a clipboard adapter without coupling tests to a
// platform window. A nil adapter models an unavailable clipboard. maxBytes
// bounds accepted results before they can reach application state.
func WithClipboard(adapter ClipboardAdapter, maxBytes int) Option {
	return func(c *config) {
		c.clipboardConfigured = true
		c.clipboardAdapter = adapter
		c.clipboardMaxBytes = maxBytes
	}
}

type idBinding struct {
	id       string
	selector Selector
}

// BindID annotates matching nodes in driver diagnostics only. Bindings are
// re-evaluated each frame and may use labels, roles and ancestry, but not ID.
// Ambiguous bindings remain ambiguous; Find and actions reject them.
func BindID(id string, selector Selector) Option {
	return func(c *config) { c.bindings = append(c.bindings, idBinding{id, selector}) }
}

// Driver owns Gio input routing and operations. All methods except Invalidate
// and Clock's read/wait methods must be called on one goroutine. A layout must
// never perform blocking I/O: the driver cannot preempt a blocked Layout call.
type Driver struct {
	config       config
	app          Harness
	router       input.Router
	ops          op.Ops
	clock        *Clock
	cancel       context.CancelFunc
	wake         chan struct{}
	closed       atomic.Bool
	closeErr     error
	frame        uint64
	wakeupAt     time.Time
	wakeup       bool
	nodes        []Node
	pressed      *pointer.Event
	clipboard    *clipboardBridge
	clipboardErr error
}

func New(root LayoutFunc, options ...Option) (*Driver, error) {
	return NewApp(func(Environment) (Harness, error) { return Harness{Layout: root}, nil }, options...)
}

// NewApp constructs the application before its first frame. Cleanup is invoked
// even on a factory error if the factory returns a partial Harness with Close.
func NewApp(create func(Environment) (Harness, error), options ...Option) (*Driver, error) {
	c := config{size: image.Pt(420, 820), metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		epoch: time.Unix(0, 0).UTC(), maxFrames: 256, interval: time.Second / 60}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("guitest: nil option")
		}
		option(&c)
	}
	if c.size.X <= 0 || c.size.Y <= 0 || !validMetric(c.metric) || c.maxFrames <= 0 || create == nil ||
		(c.clipboardConfigured && c.clipboardMaxBytes <= 0) {
		return nil, errors.New("guitest: invalid configuration")
	}
	seenIDs := make(map[string]bool)
	for _, binding := range c.bindings {
		if binding.id == "" || binding.selector == nil || seenIDs[binding.id] {
			return nil, errors.New("guitest: invalid or duplicate ID binding")
		}
		seenIDs[binding.id] = true
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := &Driver{config: c, cancel: cancel, wake: make(chan struct{}, 1)}
	d.clock = &Clock{now: c.epoch, timers: make(map[uint64]clockTimer), notify: d.Invalidate}
	if c.clipboardConfigured {
		d.clipboard = newClipboardBridge(ctx, c.clipboardAdapter, c.clipboardMaxBytes, d.Invalidate)
	}
	app, err := create(Environment{Context: ctx, Clock: d.clock, Invalidate: d.Invalidate})
	d.app = app
	if err != nil || app.Layout == nil {
		if err == nil {
			err = errors.New("guitest: nil root layout")
		}
		return nil, errors.Join(err, d.Close())
	}
	if err := d.Frame(); err != nil {
		return nil, errors.Join(err, d.Close())
	}
	return d, nil
}

func validMetric(metric unit.Metric) bool {
	for _, v := range []float32{metric.PxPerDp, metric.PxPerSp} {
		if v <= 0 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return true
}

func (d *Driver) Clock() *Clock       { return d.clock }
func (d *Driver) FrameNumber() uint64 { return d.frame }

// Render supplies the current operation list to an optional renderer without
// importing a graphics backend. The callback must not retain or mutate Ops,
// block indefinitely, or call Driver methods. Render does not advance a frame.
func (d *Driver) Render(render func(*op.Ops, image.Point) error) error {
	if d.closed.Load() {
		return ErrClosed
	}
	if render == nil {
		return errors.New("guitest: nil renderer")
	}
	return render(&d.ops, d.config.size)
}

// Invalidate coalesces asynchronous wakeups. It is safe after Close.
func (d *Driver) Invalidate() {
	if d.closed.Load() {
		return
	}
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Frame performs one real layout/Router.Frame cycle at the current virtual
// time. It does not wait for application work or advance future timers.
func (d *Driver) Frame() error {
	if d.closed.Load() {
		return ErrClosed
	}
	d.receiveClipboard()
	select {
	case <-d.wake:
	default:
	}
	d.ops.Reset()
	gtx := layout.Context{Ops: &d.ops, Source: d.router.Source(), Now: d.clock.Now(),
		Constraints: layout.Exact(d.config.size), Metric: d.config.metric, Locale: d.config.locale}
	viewport := clip.Rect{Max: d.config.size}.Push(&d.ops)
	d.app.Layout(gtx)
	viewport.Pop()
	d.router.Frame(&d.ops)
	d.sendClipboard()
	d.frame++
	d.captureNodes()
	d.wakeupAt, d.wakeup = d.router.WakeupTime()
	return nil
}

// Queue injects raw Gio events and lays out one frame to consume them. High
// level actions add the extra frames required for deferred input commands.
func (d *Driver) Queue(events ...event.Event) error {
	if d.closed.Load() {
		return ErrClosed
	}
	d.router.Queue(events...)
	return d.Frame()
}

// Advance moves virtual time forward and lays out one frame. Workers awakened
// by a clock deadline may still be running; use WaitFor/Settle to observe them.
func (d *Driver) Advance(duration time.Duration) error {
	if d.closed.Load() {
		return ErrClosed
	}
	if err := d.clock.advance(duration); err != nil {
		return err
	}
	return d.Frame()
}

func (d *Driver) Resize(width, height int, metric unit.Metric) error {
	if d.closed.Load() {
		return ErrClosed
	}
	if width <= 0 || height <= 0 || !validMetric(metric) {
		return errors.New("guitest: invalid viewport")
	}
	d.config.size, d.config.metric = image.Pt(width, height), metric
	return d.Frame()
}

func (d *Driver) due() bool { return d.wakeup && !d.wakeupAt.After(d.clock.Now()) }

// Settle waits for no immediately due redraw and application Idle. Immediate
// animation redraws advance by a virtual 1/60 second; future-only redraws (for
// example caret blinking) do not keep settling alive. Pending application
// delays need an explicit Advance. A context deadline bounds real worker waits.
func (d *Driver) Settle(ctx context.Context) error {
	return d.wait(ctx, func() bool { return !d.due() && (d.app.Idle == nil || d.app.Idle()) }, "settle")
}

// WaitFor checks a predicate after frames. It supports observing intermediate
// loading/error states without requiring the whole application to become idle.
func (d *Driver) WaitFor(ctx context.Context, predicate func() bool) error {
	if predicate == nil {
		return errors.New("guitest: nil predicate")
	}
	return d.wait(ctx, predicate, "wait")
}

func (d *Driver) wait(ctx context.Context, predicate func() bool, operation string) error {
	// A forgotten deadline must not leave a stalled worker waiting forever.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for frames := 0; frames < d.config.maxFrames; frames++ {
		if d.closed.Load() {
			return ErrClosed
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("guitest: %s at frame %d: %w", operation, d.frame, err)
		}
		if err := d.Frame(); err != nil {
			return err
		}
		// A worker may have queued a completion during this frame. Consume its
		// wakeup before accepting idle, so staged state gets a chance to apply.
		select {
		case <-d.wake:
			continue
		default:
		}
		if predicate() {
			return nil
		}
		if d.due() {
			if err := d.clock.advance(d.config.interval); err != nil {
				return err
			}
			continue
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("guitest: %s at frame %d: %w", operation, d.frame, ctx.Err())
		case <-d.wake:
		}
	}
	return fmt.Errorf("%w during %s after %d frames", ErrFrameLimit, operation, d.config.maxFrames)
}

// Close cancels Environment.Context before invoking application cleanup. It is
// idempotent; applications must join their workers in Harness.Close.
func (d *Driver) Close() error {
	if d.closed.Swap(true) {
		return d.closeErr
	}
	d.cancel()
	if d.app.Close != nil {
		d.closeErr = d.app.Close()
	}
	if d.clipboard != nil {
		d.closeErr = errors.Join(d.closeErr, d.clipboard.Close())
	}
	d.router.Frame(nil)
	d.ops.Reset()
	d.nodes = nil
	d.pressed = nil
	return d.closeErr
}
