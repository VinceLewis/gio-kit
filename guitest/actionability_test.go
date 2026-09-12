package guitest_test

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"testing"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestCheckTargetCapabilitiesIsReadOnly(t *testing.T) {
	var button widget.Clickable
	var editor widget.Editor
	var list widget.List
	list.Axis = layout.Vertical
	clicks := 0
	focused := false
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		focused = gtx.Focused(&editor)
		for button.Clicked(gtx) {
			clicks++
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Button(th, &button, "Ready tap").Layout),
			layout.Rigid(material.Editor(th, &editor, "Ready edit").Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return list.Layout(gtx, 40, func(gtx layout.Context, i int) layout.Dimensions {
					return material.Body1(th, fmt.Sprintf("Row %d", i)).Layout(gtx)
				})
			}),
		)
	}, guitest.Size(320, 480))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	frame := d.FrameNumber()
	position := list.Position
	checks := []struct {
		name       string
		selector   guitest.Selector
		capability guitest.TargetCapability
	}{
		{"tap", guitest.Label("Ready tap"), guitest.TargetTap},
		{"edit", guitest.Role(semantic.Editor), guitest.TargetEdit},
		{"scroll", guitest.Label("Row 0"), guitest.TargetScroll},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := d.CheckTarget(check.selector, check.capability); err != nil {
				t.Fatal(err)
			}
		})
	}
	if d.FrameNumber() != frame {
		t.Fatalf("readiness advanced frame: %d -> %d", frame, d.FrameNumber())
	}
	if clicks != 0 || editor.Text() != "" || focused || list.Position != position {
		t.Fatalf("readiness mutated UI: clicks=%d text=%q focused=%t position=%+v; want position=%+v",
			clicks, editor.Text(), focused, list.Position, position)
	}
}

func TestCheckTargetDistinguishesReadinessFailuresWithoutMutation(t *testing.T) {
	var ready, duplicateA, duplicateB, disabled, offViewport widget.Clickable
	clicks := 0
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		for _, button := range []*widget.Clickable{&ready, &duplicateA, &duplicateB, &disabled, &offViewport} {
			for button.Clicked(gtx) {
				clicks++
			}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Button(th, &ready, "Ready").Layout),
			layout.Rigid(material.Button(th, &duplicateA, "Duplicate").Layout),
			layout.Rigid(material.Button(th, &duplicateB, "Duplicate").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(th, &disabled, "Disabled").Layout(gtx.Disabled())
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				defer op.Offset(image.Pt(900, 0)).Push(gtx.Ops).Pop()
				return material.Button(th, &offViewport, "Off viewport").Layout(gtx)
			}),
		)
	}, guitest.Size(320, 480))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	frame := d.FrameNumber()
	tests := []struct {
		name       string
		selector   guitest.Selector
		capability guitest.TargetCapability
		want       error
	}{
		{"not found", guitest.Label("Missing"), guitest.TargetTap, guitest.ErrNotFound},
		{"ambiguous", guitest.Label("Duplicate"), guitest.TargetTap, guitest.ErrAmbiguous},
		{"disabled", guitest.Label("Disabled"), guitest.TargetTap, guitest.ErrTargetDisabled},
		{"off viewport", guitest.Label("Off viewport"), guitest.TargetTap, guitest.ErrTargetOffViewport},
		{"capability mismatch", guitest.Label("Ready"), guitest.TargetEdit, guitest.ErrTargetCapabilityMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := d.CheckTarget(test.selector, test.capability)
			if !errors.Is(err, test.want) {
				t.Fatalf("CheckTarget error = %v; want %v", err, test.want)
			}
			if test.want == guitest.ErrTargetDisabled || test.want == guitest.ErrTargetOffViewport || test.want == guitest.ErrTargetCapabilityMismatch {
				if !errors.Is(err, guitest.ErrNotInteractable) {
					t.Fatalf("specific readiness error does not wrap ErrNotInteractable: %v", err)
				}
			}
		})
	}
	if d.FrameNumber() != frame || clicks != 0 {
		t.Fatalf("failed readiness checks mutated UI: frame %d -> %d, clicks=%d", frame, d.FrameNumber(), clicks)
	}
}

func TestCheckTargetRejectsInvalidCapabilityWithoutMutation(t *testing.T) {
	var button widget.Clickable
	clicks := 0
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		for button.Clicked(gtx) {
			clicks++
		}
		return material.Button(th, &button, "Ready").Layout(gtx)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	frame := d.FrameNumber()
	err = d.CheckTarget(guitest.Label("Ready"), guitest.TargetCapability(0))
	if err == nil || !strings.Contains(err.Error(), "invalid target capability") {
		t.Fatalf("invalid capability error = %v", err)
	}
	if errors.Is(err, guitest.ErrNotInteractable) {
		t.Fatalf("invalid capability reported as target state: %v", err)
	}
	if d.FrameNumber() != frame || clicks != 0 {
		t.Fatalf("invalid capability mutated UI: frame %d -> %d, clicks=%d", frame, d.FrameNumber(), clicks)
	}
}
