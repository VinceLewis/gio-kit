package script

import (
	"gioui.org/io/semantic"
	"github.com/VinceLewis/gio-kit/guitest"
)

// CompileSelector validates and compiles a version-1 selector AST into the
// existing public guitest selector vocabulary. The returned selector retains
// no mutable state and reevaluates ancestry, occurrence, and bound IDs against
// every current frame.
func CompileSelector(selector Selector) (guitest.Selector, error) {
	terms := 0
	if err := validateSelector("selector", &selector, 1, &terms); err != nil {
		return nil, err
	}
	return compileSelector(selector), nil
}

func compileSelector(selector Selector) guitest.Selector {
	switch {
	case selector.Label != nil:
		return guitest.Label(*selector.Label)
	case selector.Description != nil:
		return guitest.Description(*selector.Description)
	case selector.Name != nil:
		return guitest.Name(*selector.Name)
	case selector.Role != nil:
		return guitest.Role(semanticRole(*selector.Role))
	case selector.Text != nil:
		return guitest.Text(*selector.Text)
	case selector.ID != nil:
		return guitest.ID(*selector.ID)
	case selector.Enabled != nil:
		return guitest.Enabled(*selector.Enabled)
	case selector.Selected != nil:
		return guitest.Selected(*selector.Selected)
	case selector.All != nil:
		selectors := make([]guitest.Selector, len(selector.All))
		for index := range selector.All {
			selectors[index] = compileSelector(selector.All[index])
		}
		return guitest.All(selectors...)
	case selector.Within != nil:
		return guitest.Within(
			compileSelector(selector.Within.Target),
			compileSelector(selector.Within.Ancestor),
		)
	case selector.Containing != nil:
		return guitest.Containing(
			compileSelector(selector.Containing.Target),
			compileSelector(selector.Containing.Descendant),
		)
	case selector.Nth != nil:
		return guitest.Nth(compileSelector(selector.Nth.Selector), selector.Nth.Index)
	default:
		// CompileSelector has already validated the closed union.
		panic("script: unreachable empty selector")
	}
}

func semanticRole(role Role) semantic.ClassOp {
	switch role {
	case RoleUnknown:
		return semantic.Unknown
	case RoleButton:
		return semantic.Button
	case RoleCheckbox:
		return semantic.CheckBox
	case RoleEditor:
		return semantic.Editor
	case RoleRadio:
		return semantic.RadioButton
	case RoleSwitch:
		return semantic.Switch
	default:
		// validateSelector rejects this before compilation.
		panic("script: unreachable selector role")
	}
}
