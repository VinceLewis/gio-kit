package shell

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/accessibility"
	"github.com/VinceLewis/gio-kit/guitest"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

func TestIconAndLabelCenterWithinAccessibleRow(t *testing.T) {
	for _, scale := range []int{1, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			theme := material.NewTheme()
			theme.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
			icon, err := widget.NewIcon(icons.ActionHome)
			if err != nil {
				t.Fatal(err)
			}
			var click widget.Clickable
			clicked := false
			d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
				for click.Clicked(gtx) {
					clicked = true
				}
				gtx.Constraints = layout.Constraints{Min: image.Pt(gtx.Dp(240), gtx.Dp(48)), Max: image.Pt(gtx.Dp(240), gtx.Dp(100))}
				return material.Clickable(gtx, &click, func(gtx layout.Context) layout.Dimensions {
					semantic.LabelOp("Aligned row").Add(gtx.Ops)
					return centeredRow(gtx, layout.UniformInset(8), func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return (accessibility.Group{Label: "Icon geometry"}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return iconLayout(gtx, icon, theme.Fg)
								})
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return (accessibility.Group{Label: "Caption geometry"}).Layout(gtx, material.Body2(theme, "Caption").Layout)
							}),
						)
					})
				})
			}, guitest.Size(320*scale, 100*scale), guitest.Metrics(unit.Metric{PxPerDp: float32(scale), PxPerSp: float32(scale)}))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = d.Close() })
			iconNode, err := d.Find(guitest.Label("Icon geometry"))
			if err != nil {
				t.Fatal(err)
			}
			caption, err := d.Find(guitest.Label("Caption geometry"))
			if err != nil {
				t.Fatal(err)
			}
			for _, bounds := range []image.Rectangle{iconNode.Desc.Bounds, caption.Desc.Bounds} {
				center := bounds.Min.Y + bounds.Dy()/2
				if center < 23*scale || center > 25*scale || bounds.Dy() > 24*scale {
					t.Fatalf("icon/text should retain intrinsic height centered in 48dp target: %v", bounds)
				}
			}
			row, err := d.Find(guitest.All(guitest.Role(semantic.Button), guitest.Name("Aligned row")))
			if err != nil || row.Desc.Bounds.Dy() != 48*scale {
				t.Fatalf("row padding enlarged its minimum target: bounds=%v err=%v", row.Desc.Bounds, err)
			}
			if err := d.Tap(guitest.All(guitest.Role(semantic.Button), guitest.Name("Aligned row"))); err != nil {
				t.Fatal(err)
			}
			if !clicked {
				t.Fatal("centered row lost its routed input target")
			}
		})
	}
}
