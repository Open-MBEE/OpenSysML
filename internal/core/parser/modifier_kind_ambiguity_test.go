package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// parseWarnings parses input as the named source and returns its warnings,
// failing the test if the parse is ill-formed.
func parseWarnings(t *testing.T, name, input string) []Diagnostic {
	t.Helper()
	p := New(source.New(name, []byte(input)))
	p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error in %q: %s", input, d.Message)
	}
	return p.Warnings
}

// A modifier, a kind keyword and no name is an anonymous usage of that kind
// (SysML.xtext Usage: UsageDeclaration? UsageCompletion); the pilot accepts it silently.
func TestAnonymousModifiedUsagesDoNotWarn(t *testing.T) {
	for _, input := range []string{
		"individual part : Vehicle;",
		"individual part : 'Gus Grissom' :> crew;",
		"individual item : Integer;",
		"individual occurrence;",
		"ref item : Integer;",
		"ref part { }",
		"snapshot part : Vehicle;",
		"timeslice item : Integer;",
		"action def A { in individual part : Vehicle; }",
		"individual part ip : Vehicle;",
		"individual ip : Vehicle;",
		"individual 'part' : Vehicle;",
		"individual : Vehicle;",
		"individual part def IP;",
		"ref x : Integer;",
		"snapshot s : Flight;",
		"snapshot part sp : Vehicle;",
		"part : Vehicle;",
		"timeslice item ts : Integer;",
	} {
		t.Run(input, func(t *testing.T) {
			if ws := parseWarnings(t, "modifier_kind.sysml", input); len(ws) != 0 {
				t.Errorf("unexpected warnings %v", ws)
			}
		})
	}
}

// The Kernel Semantic Library names features `frame` and `render`, which only
// KerML admits: SysML.xtext holds both literals, and the pinned validator
// answers `no viable alternative at input 'frame'` to `ref frame : F;`.
func TestFrameAndRenderNameKerMLFeatures(t *testing.T) {
	for _, input := range []string{"feature frame : SpatialFrame[1];", "feature render : Rendering;"} {
		t.Run(input, func(t *testing.T) {
			if ws := parseWarnings(t, "modifier_kind.kerml", input); len(ws) != 0 {
				t.Errorf("unexpected warnings %v", ws)
			}
		})
	}
}
