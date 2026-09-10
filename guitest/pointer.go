package guitest

import (
	"errors"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
)

func finitePoint(p f32.Point) bool {
	return !math.IsNaN(float64(p.X)) && !math.IsNaN(float64(p.Y)) &&
		!math.IsInf(float64(p.X), 0) && !math.IsInf(float64(p.Y), 0)
}

// Press begins a touch gesture at a semantic target. Advance and Move can
// inspect intermediate states before Release or CancelPointer ends it.
func (d *Driver) Press(selector Selector) error {
	p, err := d.target(selector, false)
	if err != nil {
		return err
	}
	return d.PressAt(p)
}

// PressAt begins a touch at viewport pixel coordinates. It intentionally has
// no semantic targeting check; use Press when a semantic target is available.
func (d *Driver) PressAt(p f32.Point) error {
	if d.closed.Load() {
		return ErrClosed
	}
	if d.pressed != nil {
		return errors.New("guitest: pointer already pressed")
	}
	if !finitePoint(p) || p.X < 0 || p.Y < 0 || p.X >= float32(d.config.size.X) || p.Y >= float32(d.config.size.Y) {
		return ErrNotInteractable
	}
	e := pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1,
		Position: p, Time: d.clock.Now().Sub(d.config.epoch)}
	d.pressed = &e
	return d.Queue(e)
}

// Move sends a held touch to pixel coordinates, including outside the viewport.
func (d *Driver) Move(p f32.Point) error {
	if d.closed.Load() {
		return ErrClosed
	}
	if d.pressed == nil {
		return errors.New("guitest: no pressed pointer")
	}
	if !finitePoint(p) {
		return errors.New("guitest: invalid pointer position")
	}
	e := *d.pressed
	e.Kind, e.Position, e.Time = pointer.Move, p, d.clock.Now().Sub(d.config.epoch)
	d.pressed = &e
	return d.Queue(e)
}

func (d *Driver) Release() error       { return d.endPointer(pointer.Release) }
func (d *Driver) CancelPointer() error { return d.endPointer(pointer.Cancel) }

func (d *Driver) endPointer(kind pointer.Kind) error {
	if d.closed.Load() {
		return ErrClosed
	}
	if d.pressed == nil {
		return errors.New("guitest: no pressed pointer")
	}
	e := *d.pressed
	e.Kind, e.Time = kind, d.clock.Now().Sub(d.config.epoch)
	d.pressed = nil
	if err := d.Queue(e); err != nil {
		return err
	}
	return d.Frame()
}

// LongPress holds a touch for an explicit virtual duration, then releases it.
func (d *Driver) LongPress(selector Selector, duration time.Duration) error {
	if duration <= 0 {
		return errors.New("guitest: long press duration must be positive")
	}
	if err := d.Press(selector); err != nil {
		return err
	}
	if err := d.Advance(duration); err != nil {
		_ = d.CancelPointer()
		return err
	}
	return d.Release()
}

func (d *Driver) DoubleTap(selector Selector) error {
	if err := d.Tap(selector); err != nil {
		return err
	}
	if err := d.Advance(50 * time.Millisecond); err != nil {
		return err
	}
	return d.Tap(selector)
}

// Drag moves a touch by delta pixels over virtual duration. Steps are bounded
// by MaxFrames. Gio decides drag ownership and fling behavior.
func (d *Driver) Drag(selector Selector, delta f32.Point, duration time.Duration) error {
	if !finitePoint(delta) || duration <= 0 {
		return errors.New("guitest: invalid drag")
	}
	steps := duration/d.config.interval + 1
	if steps < 4 {
		steps = 4
	}
	if steps > time.Duration(d.config.maxFrames) {
		return ErrFrameLimit
	}
	if err := d.Press(selector); err != nil {
		return err
	}
	origin := d.pressed.Position
	var elapsed time.Duration
	for step := time.Duration(1); step <= steps; step++ {
		next := time.Duration(float64(duration) * float64(step) / float64(steps))
		if err := d.clock.advance(next - elapsed); err != nil {
			_ = d.CancelPointer()
			return err
		}
		elapsed = next
		p := origin.Add(delta.Mul(float32(step) / float32(steps)))
		if err := d.Move(p); err != nil {
			_ = d.CancelPointer()
			return err
		}
	}
	return d.Release()
}

// Scroll sends a mouse-wheel event to the target's nearest scrollable
// ancestor. Positive Y moves toward later rows. Delta is in physical pixels.
// Use Drag to exercise touch scrolling and its gesture arbitration.
func (d *Driver) Scroll(selector Selector, delta f32.Point) error {
	if !finitePoint(delta) {
		return errors.New("guitest: invalid scroll delta")
	}
	if d.pressed != nil {
		return errors.New("guitest: pointer already pressed")
	}
	p, err := d.targetKind(selector, false, true)
	if err != nil {
		return err
	}
	if err := d.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse,
		Position: p, Scroll: delta, Time: d.clock.Now().Sub(d.config.epoch)}); err != nil {
		return err
	}
	return d.Frame()
}
