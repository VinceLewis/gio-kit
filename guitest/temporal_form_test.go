package guitest_test

import (
	"testing"

	"gioui.org/layout"
	formkit "github.com/VinceLewis/gio-kit/form"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestTemporalPickerAppliesAndCancelsWithoutTextEntry(t *testing.T) {
	form, err := formkit.New([]formkit.FieldSchema{{ID: "day", Label: "Day", Type: formkit.FieldDate}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer form.Close()
	if err := form.Load(map[string]string{"day": "2026-09-10"}); err != nil {
		t.Fatal(err)
	}
	widget := formkit.NewWidget(form)
	theme := theme()
	driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return widget.Layout(gtx, theme) })
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Tap(guitest.Label("Pick date")); err != nil {
		t.Fatal(err)
	}
	if err := driver.Tap(guitest.Label("+ DAY")); err != nil {
		t.Fatal(err)
	}
	if err := driver.Tap(guitest.Label("USE VALUE")); err != nil {
		t.Fatal(err)
	}
	if got := form.Values()["day"]; got != "2026-09-11" {
		t.Fatalf("picked day = %q", got)
	}
	if err := driver.Tap(guitest.Label("Pick date")); err != nil {
		t.Fatal(err)
	}
	if err := driver.Tap(guitest.Label("+ DAY")); err != nil {
		t.Fatal(err)
	}
	if err := driver.Tap(guitest.Label("CANCEL PICKER")); err != nil {
		t.Fatal(err)
	}
	if got := form.Values()["day"]; got != "2026-09-11" {
		t.Fatalf("cancelled day = %q", got)
	}
}
