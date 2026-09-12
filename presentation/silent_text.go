package presentation

import (
	"image"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget/material"

	"golang.org/x/image/math/fixed"
)

// silentLabel paints wrapped, truncated text pixel-for-pixel like
// material.LabelStyle.Layout, but never adds a semantic node of its own.
//
// gioui.org/widget.Label unconditionally emits its own semantic.LabelOp
// inside a freshly pushed clip area, and Gio's semantic tree treats every
// clip push as a node boundary — so every material.Label call becomes an
// independent leaf. That is correct for a standalone label, but wrong for
// one fragment of a feed row's ordered rich-text run, whose complete
// accessible description already lives on the row's own
// accessibility.Group. Gio elides (flattens away) a clip area that never
// receives any semantic op — its children reparent to the nearest ancestor
// that has one — so painting through this copy, identical except for the
// omitted semantic.LabelOp call, leaves the fragment's glyphs on screen
// without adding a node for them; they read as part of the enclosing row
// group instead of as their own announced item.
//
// Ported from gioui.org/widget.Label (SPDX: Unlicense OR MIT); the only
// change from the original is the removed semantic.LabelOp call.
func silentLabel(gtx layout.Context, style material.LabelStyle) layout.Dimensions {
	if style.Shaper == nil {
		return layout.Dimensions{}
	}
	cs := gtx.Constraints
	textSize := fixed.I(gtx.Sp(style.TextSize))
	lineHeight := fixed.I(gtx.Sp(style.LineHeight))
	style.Shaper.LayoutString(text.Parameters{
		Font:            style.Font,
		PxPerEm:         textSize,
		MaxLines:        style.MaxLines,
		Truncator:       style.Truncator,
		Alignment:       style.Alignment,
		WrapPolicy:      style.WrapPolicy,
		MaxWidth:        cs.Max.X,
		MinWidth:        cs.Min.X,
		Locale:          gtx.Locale,
		LineHeight:      lineHeight,
		LineHeightScale: style.LineHeightScale,
	}, style.Text)

	colorMacro := op.Record(gtx.Ops)
	paint.ColorOp{Color: style.Color}.Add(gtx.Ops)
	textColor := colorMacro.Stop()

	m := op.Record(gtx.Ops)
	viewport := image.Rectangle{Max: cs.Max}
	it := silentTextIterator{
		viewport: viewport,
		maxLines: style.MaxLines,
		material: textColor,
	}
	var glyphs [32]text.Glyph
	line := glyphs[:0]
	for g, ok := style.Shaper.NextGlyph(); ok; g, ok = style.Shaper.NextGlyph() {
		var ok bool
		if line, ok = it.paintGlyph(gtx, style.Shaper, g, line); !ok {
			break
		}
	}
	call := m.Stop()
	viewport.Min = viewport.Min.Add(it.padding.Min)
	viewport.Max = viewport.Max.Add(it.padding.Max)
	clipStack := clip.Rect(viewport).Push(gtx.Ops)
	call.Add(gtx.Ops)
	dims := layout.Dimensions{Size: it.bounds.Size()}
	dims.Size = cs.Constrain(dims.Size)
	dims.Baseline = dims.Size.Y - it.baseline
	clipStack.Pop()
	return dims
}

// silentTextIterator is textIterator (gioui.org/widget) verbatim, minus the
// semantic.LabelOp call silentLabel omits above. Kept as a private copy
// because the original type is unexported in gioui.org/widget.
type silentTextIterator struct {
	viewport  image.Rectangle
	maxLines  int
	material  op.CallOp
	truncated int
	linesSeen int
	lineOff   f32.Point
	padding   image.Rectangle
	bounds    image.Rectangle
	visible   bool
	first     bool
	baseline  int
}

func (it *silentTextIterator) processGlyph(g text.Glyph, ok bool) (visibleOrBefore bool) {
	if it.maxLines > 0 {
		if g.Flags&text.FlagTruncator != 0 && g.Flags&text.FlagClusterBreak != 0 {
			it.truncated = int(g.Runes)
		}
		if g.Flags&text.FlagLineBreak != 0 {
			it.linesSeen++
		}
		if it.linesSeen == it.maxLines && g.Flags&text.FlagParagraphBreak != 0 {
			return false
		}
	}
	if d := g.Bounds.Min.X.Floor(); d < it.padding.Min.X {
		it.padding.Min.X = d
	}
	if d := (g.Bounds.Max.X - g.Advance).Ceil(); d > it.padding.Max.X {
		it.padding.Max.X = d
	}
	if d := (g.Bounds.Min.Y + g.Ascent).Floor(); d < it.padding.Min.Y {
		it.padding.Min.Y = d
	}
	if d := (g.Bounds.Max.Y - g.Descent).Ceil(); d > it.padding.Max.Y {
		it.padding.Max.Y = d
	}
	logicalBounds := image.Rectangle{
		Min: image.Pt(g.X.Floor(), int(g.Y)-g.Ascent.Ceil()),
		Max: image.Pt((g.X + g.Advance).Ceil(), int(g.Y)+g.Descent.Ceil()),
	}
	if !it.first {
		it.first = true
		it.baseline = int(g.Y)
		it.bounds = logicalBounds
	}
	above := logicalBounds.Max.Y < it.viewport.Min.Y
	below := logicalBounds.Min.Y > it.viewport.Max.Y
	left := logicalBounds.Max.X < it.viewport.Min.X
	right := logicalBounds.Min.X > it.viewport.Max.X
	it.visible = !above && !below && !left && !right
	if it.visible {
		it.bounds.Min.X = min(it.bounds.Min.X, logicalBounds.Min.X)
		it.bounds.Min.Y = min(it.bounds.Min.Y, logicalBounds.Min.Y)
		it.bounds.Max.X = max(it.bounds.Max.X, logicalBounds.Max.X)
		it.bounds.Max.Y = max(it.bounds.Max.Y, logicalBounds.Max.Y)
	}
	return ok && !below
}

func silentFixedToFloat(i fixed.Int26_6) float32 {
	return float32(i) / 64.0
}

func (it *silentTextIterator) paintGlyph(gtx layout.Context, shaper *text.Shaper, glyph text.Glyph, line []text.Glyph) ([]text.Glyph, bool) {
	visibleOrBefore := it.processGlyph(glyph, true)
	if it.visible {
		if len(line) == 0 {
			it.lineOff = f32.Point{X: silentFixedToFloat(glyph.X), Y: float32(glyph.Y)}.Sub(layout.FPt(it.viewport.Min))
		}
		line = append(line, glyph)
	}
	if glyph.Flags&text.FlagLineBreak != 0 || cap(line)-len(line) == 0 || !visibleOrBefore {
		t := op.Affine(f32.AffineId().Offset(it.lineOff)).Push(gtx.Ops)
		path := shaper.Shape(line)
		outline := clip.Outline{Path: path}.Op().Push(gtx.Ops)
		it.material.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		outline.Pop()
		if call := shaper.Bitmaps(line); call != (op.CallOp{}) {
			call.Add(gtx.Ops)
		}
		t.Pop()
		line = line[:0]
	}
	return line, visibleOrBefore
}
