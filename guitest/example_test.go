package guitest_test

import (
	"fmt"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
)

func ExampleDriver_Tap() {
	var button widget.Clickable
	clicks := 0
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		for button.Clicked(gtx) {
			clicks++
		}
		return material.Button(th, &button, "Save").Layout(gtx)
	}, guitest.Size(420, 820))
	if err != nil {
		panic(err)
	}
	defer driver.Close()
	if err := driver.Tap(guitest.Label("Save")); err != nil {
		panic(err)
	}
	fmt.Println(clicks)
	// Output: 1
}
