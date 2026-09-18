package screenshot

import (
	"errors"
	"image"
	"image/color"

	"github.com/VinceLewis/gio-kit/guitest"
)

// NodeColor is a per-node color estimate from a rendered frame (see
// NodeColors' doc comment for why this samples pixels rather than
// instrumenting paint ops, the approach gio-json-test-upgrade-plan.md item 4
// originally specified).
type NodeColor struct {
	// Background is the modal color of the node's own border pixels (its
	// Bounds rectangle's edge, 1px in from each side): the pixels least
	// likely to be covered by the node's own foreground content (text
	// glyphs, an icon's ink) and most likely to be its fill/background.
	Background color.NRGBA
	// Foreground is the modal color among the node's interior pixels that
	// differs from Background, or equal to Background when the interior is
	// uniform (a plain-filled node with no distinct foreground content,
	// e.g. an unlabeled color swatch).
	Foreground color.NRGBA
}

// NodeColors estimates Background/Foreground for every node whose Bounds are
// non-empty and fully within img (typically the result of capture(d) for the
// same frame the nodes were captured from).
//
// gio-json-test-upgrade-plan.md item 4 originally specified reading
// paint.ColorOp/paint.LinearGradientOp directly at capture time. That is not
// implementable through Gio's public API: op.Ops has no exported reader, and
// gioui.org/io/input.SemanticDesc carries no tag or op-offset linking a
// semantic node back to the paint ops that drew it (confirmed by inspecting
// gioui.org's io/input and op packages — the offset/tag correlation lives
// only in Gio's internal ops decoder). Pixel-sampling the actual rendered
// frame is the closest achievable equivalent: it needs no Gio internals,
// reuses the same headless GPU path Save/WritePNG already use, and reflects
// the true composited color (after theme, opacity and gradients) rather than
// a single raw ColorOp, which is arguably the more useful signal for a test
// asking "does this look right" anyway. The tradeoff is precision: this is a
// statistical estimate over a rectangle, not an exact color read, so a node
// with low-contrast or highly detailed content (e.g. a photo) may produce a
// noisy Foreground estimate. Treat NodeColors as a coarse regression signal
// (did the theme change unexpectedly?), not a color-correctness oracle.
func NodeColors(img image.Image, nodes []guitest.FrameNode) (map[string]NodeColor, error) {
	if img == nil {
		return nil, errors.New("guitest: nil image")
	}
	result := make(map[string]NodeColor, len(nodes))
	imgBounds := img.Bounds()
	for _, n := range nodes {
		r := image.Rect(n.Bounds.X, n.Bounds.Y, n.Bounds.X+n.Bounds.Width, n.Bounds.Y+n.Bounds.Height)
		if r.Dx() <= 0 || r.Dy() <= 0 || !r.In(imgBounds) {
			continue
		}
		background := modalColor(img, borderPixels(r))
		interior := r.Inset(1)
		var foreground color.NRGBA
		if interior.Dx() > 0 && interior.Dy() > 0 {
			foreground = modalColorExcluding(img, interiorPixels(interior), background)
		}
		if (foreground == color.NRGBA{}) {
			foreground = background
		}
		result[n.ID] = NodeColor{Background: background, Foreground: foreground}
	}
	return result, nil
}

func borderPixels(r image.Rectangle) func(func(image.Point) bool) {
	return func(yield func(image.Point) bool) {
		for x := r.Min.X; x < r.Max.X; x++ {
			if !yield(image.Pt(x, r.Min.Y)) || !yield(image.Pt(x, r.Max.Y-1)) {
				return
			}
		}
		for y := r.Min.Y + 1; y < r.Max.Y-1; y++ {
			if !yield(image.Pt(r.Min.X, y)) || !yield(image.Pt(r.Max.X-1, y)) {
				return
			}
		}
	}
}

func interiorPixels(r image.Rectangle) func(func(image.Point) bool) {
	return func(yield func(image.Point) bool) {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				if !yield(image.Pt(x, y)) {
					return
				}
			}
		}
	}
}

func toNRGBA(img image.Image, p image.Point) color.NRGBA {
	r, g, b, a := img.At(p.X, p.Y).RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func modalColor(img image.Image, points func(func(image.Point) bool)) color.NRGBA {
	return mode(colorSequence(img, points, color.NRGBA{}, false))
}

func modalColorExcluding(img image.Image, points func(func(image.Point) bool), exclude color.NRGBA) color.NRGBA {
	return mode(colorSequence(img, points, exclude, true))
}

// colorSequence returns pixel colors in scan order, so mode's tie-break
// (first color reached in that order) is deterministic across runs: a Go map
// iterates in randomized order, so picking the "first" max-count key
// directly from a map would make an exact-tie result flaky.
func colorSequence(img image.Image, points func(func(image.Point) bool), exclude color.NRGBA, excluding bool) []color.NRGBA {
	var seq []color.NRGBA
	points(func(p image.Point) bool {
		c := toNRGBA(img, p)
		if !excluding || c != exclude {
			seq = append(seq, c)
		}
		return true
	})
	return seq
}

func mode(seq []color.NRGBA) color.NRGBA {
	counts := make(map[color.NRGBA]int, len(seq))
	var best color.NRGBA
	bestCount := 0
	for _, c := range seq {
		counts[c]++
		if counts[c] > bestCount {
			best, bestCount = c, counts[c]
		}
	}
	return best
}
