package script_test

import (
	"errors"
	"fmt"
	"image"
	"testing"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/accessibility"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/script"
)

func TestCompileSelectorParityAcrossFrames(t *testing.T) {
	driver, reversed := selectorDriver(t)

	labelSave := textPointer("Save")
	groupOne := textPointer("Group 1")
	groupZero := textPointer("Group 0")
	description := textPointer("Description 0")
	button := rolePointer(script.RoleButton)
	disabled := boolPointer(false)
	selected := boolPointer(true)
	id := textPointer("second.save")
	tables := []struct {
		name   string
		ast    script.Selector
		direct guitest.Selector
	}{
		{"label", script.Selector{Label: labelSave}, guitest.Label("Save")},
		{"description", script.Selector{Description: description}, guitest.Description("Description 0")},
		{"name", script.Selector{Name: groupZero}, guitest.Name("Group 0")},
		{"role", script.Selector{Role: button}, guitest.Role(semantic.Button)},
		{"text", script.Selector{Text: textPointer("scription 0")}, guitest.Text("scription 0")},
		{"id", script.Selector{ID: id}, guitest.ID("second.save")},
		{"enabled inherited", script.Selector{Enabled: disabled}, guitest.Enabled(false)},
		{"selected", script.Selector{Selected: selected}, guitest.Selected(true)},
		{"all and inherited name", script.Selector{All: []script.Selector{
			{Role: button}, {Name: groupOne}, {Enabled: disabled},
		}}, guitest.All(guitest.Role(semantic.Button), guitest.Name("Group 1"), guitest.Enabled(false))},
		{"within", script.Selector{Within: &script.WithinSelector{
			Target: script.Selector{Label: labelSave}, Ancestor: script.Selector{Name: groupOne},
		}}, guitest.Within(guitest.Label("Save"), guitest.Name("Group 1"))},
		{"containing", script.Selector{Containing: &script.ContainingSelector{
			Target: script.Selector{Name: groupOne}, Descendant: script.Selector{Label: labelSave},
		}}, guitest.Containing(guitest.Name("Group 1"), guitest.Label("Save"))},
		{"nth", script.Selector{Nth: &script.NthSelector{
			Selector: script.Selector{Label: labelSave}, Index: 1,
		}}, guitest.Nth(guitest.Label("Save"), 1)},
	}

	for _, reverse := range []bool{false, true} {
		*reversed = reverse
		width := 420
		if reverse {
			width = 640
		}
		if err := driver.Resize(width, 500, unit.Metric{PxPerDp: 1, PxPerSp: 1}); err != nil {
			t.Fatal(err)
		}
		for _, test := range tables {
			t.Run(fmt.Sprintf("%s/reversed=%t", test.name, reverse), func(t *testing.T) {
				compiled, err := script.CompileSelector(test.ast)
				if err != nil {
					t.Fatal(err)
				}
				assertSelectorParity(t, driver, compiled, test.direct)
				if countMatches(driver, compiled) == 0 {
					t.Fatal("test layout did not exercise selector")
				}
			})
		}
	}
}

func TestCompileSelectorMapsEveryRole(t *testing.T) {
	driver, _ := selectorDriver(t)
	tests := []struct {
		role   script.Role
		direct semantic.ClassOp
	}{
		{script.RoleUnknown, semantic.Unknown},
		{script.RoleButton, semantic.Button},
		{script.RoleCheckbox, semantic.CheckBox},
		{script.RoleEditor, semantic.Editor},
		{script.RoleRadio, semantic.RadioButton},
		{script.RoleSwitch, semantic.Switch},
	}
	for _, test := range tests {
		t.Run(string(test.role), func(t *testing.T) {
			compiled, err := script.CompileSelector(script.Selector{Role: rolePointer(test.role)})
			if err != nil {
				t.Fatal(err)
			}
			direct := guitest.Role(test.direct)
			assertSelectorParity(t, driver, compiled, direct)
			if countMatches(driver, compiled) == 0 {
				t.Fatal("test layout did not exercise role")
			}
		})
	}
}

func TestCompileSelectorPreservesFindErrorsAndOccurrence(t *testing.T) {
	driver, reversed := selectorDriver(t)
	duplicate, err := script.CompileSelector(script.Selector{Label: textPointer("Save")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = driver.Find(duplicate); !errors.Is(err, guitest.ErrAmbiguous) {
		t.Fatalf("duplicate Find error = %v", err)
	}
	missing, err := script.CompileSelector(script.Selector{ID: textPointer("not-bound")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = driver.Find(missing); !errors.Is(err, guitest.ErrNotFound) {
		t.Fatalf("missing Find error = %v", err)
	}

	nthAST := script.Selector{Nth: &script.NthSelector{Selector: script.Selector{Label: textPointer("Save")}, Index: 1}}
	nth, err := script.CompileSelector(nthAST)
	if err != nil {
		t.Fatal(err)
	}
	first := mustFind(t, driver, nth)
	if !guitest.Within(nth, guitest.Name("Group 1"))(first) {
		t.Fatal("initial nth selector did not select the second logical group")
	}
	*reversed = true
	if err = driver.Resize(640, 500, unit.Metric{PxPerDp: 1, PxPerSp: 1}); err != nil {
		t.Fatal(err)
	}
	second := mustFind(t, driver, nth)
	if !guitest.Within(nth, guitest.Name("Group 0"))(second) {
		t.Fatal("nth selector did not reevaluate against reordered frame")
	}

	bound, err := script.CompileSelector(script.Selector{ID: textPointer("second.save")})
	if err != nil {
		t.Fatal(err)
	}
	directBound := guitest.ID("second.save")
	assertSelectorParity(t, driver, bound, directBound)
	if _, err = driver.Find(bound); err != nil {
		t.Fatalf("bound ID after resize/reorder: %v", err)
	}
}

func TestCompileSelectorRejectsInvalidAST(t *testing.T) {
	name := "x"
	role := script.RoleButton
	deep := script.Selector{Name: &name}
	for range script.DefaultSelectorDepth {
		deep = script.Selector{Nth: &script.NthSelector{Selector: deep}}
	}
	tests := map[string]script.Selector{
		"empty":              {},
		"multiple operators": {Name: &name, Role: &role},
		"empty all":          {All: []script.Selector{}},
		"negative nth":       {Nth: &script.NthSelector{Selector: script.Selector{Name: &name}, Index: -1}},
		"unknown role":       {Role: rolePointer(script.Role("link"))},
		"too deep":           deep,
	}
	for name, ast := range tests {
		t.Run(name, func(t *testing.T) {
			compiled, err := script.CompileSelector(ast)
			if compiled != nil || !errors.Is(err, script.ErrInvalidScript) {
				t.Fatalf("CompileSelector = (%v, %v)", compiled, err)
			}
		})
	}
}

func selectorDriver(t *testing.T) (*guitest.Driver, *bool) {
	t.Helper()
	var buttons [2]widget.Clickable
	reversed := false
	theme := material.NewTheme()
	driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		order := [2]int{0, 1}
		if reversed {
			order = [2]int{1, 0}
		}
		children := make([]layout.FlexChild, 0, 7)
		for _, index := range order {
			index := index
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				group := accessibility.Group{
					Label:       fmt.Sprintf("Group %d", index),
					Description: fmt.Sprintf("Description %d", index),
					Disabled:    index == 1,
					Selected:    index == 0,
				}
				return group.Layout(gtx, material.Button(theme, &buttons[index], "Save").Layout)
			}))
		}
		roles := []struct {
			class semantic.ClassOp
			label string
		}{
			{semantic.CheckBox, "Checkbox role"},
			{semantic.Editor, "Editor role"},
			{semantic.RadioButton, "Radio role"},
			{semantic.Switch, "Switch role"},
		}
		for _, role := range roles {
			role := role
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				defer clip.Rect(image.Rect(0, 0, 120, 24)).Push(gtx.Ops).Pop()
				semantic.ClassOp(role.class).Add(gtx.Ops)
				semantic.LabelOp(role.label).Add(gtx.Ops)
				return layout.Dimensions{Size: image.Pt(120, 24)}
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}, guitest.BindID("second.save", guitest.Within(guitest.Label("Save"), guitest.Name("Group 1"))))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	return driver, &reversed
}

func assertSelectorParity(t *testing.T, driver *guitest.Driver, compiled, direct guitest.Selector) {
	t.Helper()
	nodes := driver.Nodes()
	for _, node := range nodes {
		if got, want := compiled(node), direct(node); got != want {
			t.Fatalf("node %d parity = %t, want %t", node.Index, got, want)
		}
	}
	got, gotErr := driver.Find(compiled)
	want, wantErr := driver.Find(direct)
	if errorClass(gotErr) != errorClass(wantErr) {
		t.Fatalf("Find errors differ: compiled=%v direct=%v", gotErr, wantErr)
	}
	if gotErr == nil && got.Index != want.Index {
		t.Fatalf("Find indices differ: compiled=%d direct=%d", got.Index, want.Index)
	}
}

func errorClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, guitest.ErrNotFound):
		return "not_found"
	case errors.Is(err, guitest.ErrAmbiguous):
		return "ambiguous"
	default:
		return err.Error()
	}
}

func countMatches(driver *guitest.Driver, selector guitest.Selector) int {
	count := 0
	for _, node := range driver.Nodes() {
		if selector(node) {
			count++
		}
	}
	return count
}

func mustFind(t *testing.T, driver *guitest.Driver, selector guitest.Selector) guitest.Node {
	t.Helper()
	node, err := driver.Find(selector)
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func textPointer(value string) *string           { return &value }
func rolePointer(value script.Role) *script.Role { return &value }
func boolPointer(value bool) *bool               { return &value }
