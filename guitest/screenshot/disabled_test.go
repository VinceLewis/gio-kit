//go:build !guitestgpu

package screenshot_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/screenshot"
)

func TestUnavailableCaptureRemovesStalePNG(t *testing.T) {
	d, err := guitest.New(func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	base := filepath.Join(t.TempDir(), "frame")
	if err = os.WriteFile(base+".png", []byte("previous run"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = screenshot.Save(d, base, guitest.DumpOptions{}); !errors.Is(err, screenshot.ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err = os.Stat(base + ".png"); !os.IsNotExist(err) {
		t.Fatal("old PNG remained paired with a new tree")
	}
	if _, err = os.Stat(base + ".json"); err != nil {
		t.Fatal(err)
	}
}
