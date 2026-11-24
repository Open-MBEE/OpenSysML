package runtime

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessVerbatimEquation covers the failure modes of a
// constraint body that carries equations verbatim as annotation `body` Strings:
// the text is data, never an expression the runtime tries to resolve, and its
// absence as a condition is a typed verdict failure rather than a panic.
func TestRuntimeRobustnessVerbatimEquation(t *testing.T) {
	t.Run("annotations_alone_state_no_condition", testVerbatimEquationAnnotationsAloneStateNoCondition)
	t.Run("body_names_are_not_resolved", testVerbatimEquationBodyNamesAreNotResolved)
	t.Run("hostile_body_is_read_verbatim", testVerbatimEquationHostileBodyIsReadVerbatim)
}

func testVerbatimEquationAnnotationsAloneStateNoCondition(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			metadata def EquationMapping { attribute body : String; }
			part def Rig {
				constraint onlyEquations {
					@EquationMapping { body = "Medium.temperature(state_a)"; }
					@EquationMapping { body = "inStream(port_a.h_outflow)"; }
				}
			}
		}
	`
	ctx, testPkg := constraintPackage(t, src)
	rig, ok := testPkg.LookupLocal("Rig")
	if !ok {
		t.Fatal("Rig not found")
	}
	feat := featureNamed(ctx, rig, "onlyEquations")
	if feat == nil || feat.Symbol == nil {
		t.Fatal("constraint feature not found")
	}

	satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), nil)
	if !errors.Is(err, ErrNoValue) || !strings.Contains(err.Error(), "declares no condition") {
		t.Fatalf("err = %v, want ErrNoValue naming the missing condition", err)
	}
	if satisfied {
		t.Error("a body carrying only verbatim equations reported as satisfied")
	}
}

func testVerbatimEquationBodyNamesAreNotResolved(t *testing.T) {
	// The body names `Medium`, `inStream` and `noEvent`, none of which the
	// model declares. They are text in an annotation and must not become
	// unresolved references; the body's own condition still decides.
	src := `
		package test {
			private import ScalarValues::*;
			metadata def EquationMapping { attribute body : String; }
			part def Rig {
				attribute m_flow : Real = 2.0;
				constraint withEquations {
					@EquationMapping { body = "Medium.temperature(state_a)"; }
					@EquationMapping { body = "inStream(port_a.h_outflow)"; }
					@EquationMapping { body = "noEvent(if abs(m_flow) > 1.0 then m_flow else 1.0)"; }
					m_flow > 0.0
				}
			}
		}
	`
	ctx, testPkg := constraintPackage(t, src)
	rig, ok := testPkg.LookupLocal("Rig")
	if !ok {
		t.Fatal("Rig not found")
	}
	feat := featureNamed(ctx, rig, "withEquations")
	if feat == nil || feat.Symbol == nil {
		t.Fatal("constraint feature not found")
	}

	satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), nil)
	if err != nil {
		t.Fatalf("a verbatim equation body must not fail evaluation: %v", err)
	}
	if !satisfied {
		t.Error("the body's own condition was not decided")
	}
}

func testVerbatimEquationHostileBodyIsReadVerbatim(t *testing.T) {
	// Quotes, a backslash, an ampersand, angle brackets and a `then` conditional
	// are characters in the annotation's string, not notation the engine parses.
	// The body is carried into the AST exactly as written, with no resolution of
	// the names it mentions and no panic on its escape sequences.
	const equation = `q = "a\b" & <x> noEvent(if n > 1 then n else 1)`
	src := `
		package test {
			private import ScalarValues::*;
			metadata def EquationMapping { attribute body : String; }
			part def Rig {
				constraint c {
					@EquationMapping { body = "q = \"a\\b\" & <x> noEvent(if n > 1 then n else 1)"; }
					true
				}
			}
		}
	`
	ctx, testPkg := constraintPackage(t, src)
	rig, ok := testPkg.LookupLocal("Rig")
	if !ok {
		t.Fatal("Rig not found")
	}
	feat := featureNamed(ctx, rig, "c")
	if feat == nil || feat.Symbol == nil || feat.Symbol.Decl == nil {
		t.Fatal("c constraint not found")
	}
	found := false
	ast.Inspect(feat.Symbol.Decl, func(n ast.Node) bool {
		if literal, ok := n.(*ast.LiteralString); ok {
			if unquoted, err := strconv.Unquote(literal.Value); err == nil && unquoted == equation {
				found = true
			}
		}
		return true
	})
	if !found {
		t.Errorf("the annotation did not carry %q verbatim into the AST", equation)
	}
}
