//go:build !guitestgpu

package screenshot

import (
	"github.com/VinceLewis/gio-kit/guitest"
	"image"
)

func capture(*guitest.Driver) (image.Image, error) { return nil, ErrUnavailable }
