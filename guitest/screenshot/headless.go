//go:build guitestgpu

package screenshot

import (
	"fmt"
	"gioui.org/gpu/headless"
	"gioui.org/op"
	"github.com/VinceLewis/gio-kit/guitest"
	"image"
)

func capture(d *guitest.Driver) (image.Image, error) {
	var img *image.RGBA
	err := d.Render(func(ops *op.Ops, size image.Point) error {
		window, err := headless.NewWindow(size.X, size.Y)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		defer window.Release()
		if err := window.Frame(ops); err != nil {
			return err
		}
		img = image.NewRGBA(image.Rectangle{Max: size})
		return window.Screenshot(img)
	})
	return img, err
}
