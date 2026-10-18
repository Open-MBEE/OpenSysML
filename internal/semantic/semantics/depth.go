package semantics

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A view's rendering depth is a non-normative convention, not a specification
// rule: an attribute usage named depth on a view, bound to a non-negative
// integer literal, bounds how many levels of nested elements a rendering draws
// below each exposed element. 0, or no depth at all, draws every level.

// DepthAttribute is the name of the attribute that states a view's depth.
const DepthAttribute = "depth"

// DepthProblem is a depth attribute whose value is not a non-negative integer
// literal, located at the value.
type DepthProblem struct {
	Node    ast.Node
	Message string
}

// ViewDepth is the depth view states: the depth attribute of the view itself,
// else of the nearest view it specializes that binds one. 0 when none does,
// and for a value that cannot be read (see ViewDepthProblems).
func (m *Model) ViewDepth(view *symbols.Symbol) int {
	if view == nil || !IsView(view) {
		return 0
	}
	for _, owner := range append([]*symbols.Symbol{view}, m.AllSupertypes(view)...) {
		if !IsView(owner) {
			continue
		}
		for _, member := range depthMembers(owner) {
			if value := member.Decl.(*ast.Usage).Value; value != nil {
				depth, _ := depthValue(value)
				return depth
			}
		}
	}
	return 0
}

// ViewDepthProblems is each depth attribute view itself binds to anything but a
// non-negative integer literal.
func (m *Model) ViewDepthProblems(view *symbols.Symbol) []DepthProblem {
	if view == nil || !IsView(view) {
		return nil
	}
	var out []DepthProblem
	for _, member := range depthMembers(view) {
		value := member.Decl.(*ast.Usage).Value
		if value == nil {
			continue
		}
		if _, ok := depthValue(value); !ok {
			out = append(out, DepthProblem{Node: value, Message: fmt.Sprintf(
				"%s is bound to no non-negative integer literal; a view's depth is how many levels of nested elements its rendering draws",
				DepthAttribute)})
		}
	}
	return out
}

// depthMembers is each attribute usage named depth a view declares, a
// redefinition `:>> depth = 2` of an inherited one included.
func depthMembers(view *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, member := range usageMembersOfKind(view, ast.UsageAttribute) {
		if member.Name == DepthAttribute {
			out = append(out, member)
		}
	}
	return out
}

// depthValue reads a bound depth: a non-negative integer literal.
func depthValue(value ast.Node) (int, bool) {
	literal, ok := value.(*ast.LiteralInteger)
	if !ok {
		return 0, false
	}
	depth, err := strconv.Atoi(literal.Value)
	if err != nil || depth < 0 {
		return 0, false
	}
	return depth, true
}
