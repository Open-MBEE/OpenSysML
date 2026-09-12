package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// nested walks a chain of member names from a scope and returns the symbol the
// last one names.
func nested(t *testing.T, scope *symbols.Scope, path ...string) *symbols.Symbol {
	t.Helper()
	var found *symbols.Symbol
	for _, name := range path {
		if scope == nil {
			t.Fatalf("no scope while looking up %q", name)
		}
		s, ok := scope.LookupLocal(name)
		if !ok {
			t.Fatalf("symbol %q not found", name)
		}
		found, scope = s, s.Scope
	}
	return found
}

// TestImplicitParameterRedefinitionByPosition covers the core rule of KerML
// 7.4.7.2/7.4.7.3: a parameter of a step redefines the parameter at the same
// position of the behavior that types it, and so takes its type.
func TestImplicitParameterRedefinitionByPosition(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Scene;
		part def Image;
		action def Focus { in scene : Scene; out image : Image; }
		action focus : Focus { in item scene; out item image; }
	}`)
	p := sym(t, root, "P")
	focusDef := nested(t, p.Scope, "Focus")
	wantScene := nested(t, focusDef.Scope, "scene")
	wantImage := nested(t, focusDef.Scope, "image")
	image := nested(t, p.Scope, "focus", "image")

	if supers := m.DirectSupertypes(nested(t, p.Scope, "focus", "scene")); len(supers) != 1 || supers[0] != wantScene {
		t.Fatalf("DirectSupertypes(focus::scene) = %v, want [Focus::scene]", supers)
	}
	if supers := m.DirectSupertypes(image); len(supers) != 1 || supers[0] != wantImage {
		t.Fatalf("DirectSupertypes(focus::image) = %v, want [Focus::image]", supers)
	}
	// The redefined parameter supplies the type, so its members are reachable.
	if !m.Conforms(image, nested(t, p.Scope, "Image")) {
		t.Fatalf("focus::image does not conform to Image through the redefined parameter")
	}
}

// TestImplicitParameterRedefinitionPositionNotName covers that the match is
// positional, not by name: a renamed parameter still redefines the parameter at
// its own position.
func TestImplicitParameterRedefinitionPositionNotName(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Scene;
		part def Image;
		action def Focus { in scene : Scene; out image : Image; }
		action focus : Focus { in item lighting; out item picture; }
	}`)
	p := sym(t, root, "P")
	focusDef := nested(t, p.Scope, "Focus")

	if supers := m.DirectSupertypes(nested(t, p.Scope, "focus", "lighting")); len(supers) != 1 ||
		supers[0] != nested(t, focusDef.Scope, "scene") {
		t.Fatalf("DirectSupertypes(focus::lighting) = %v, want [Focus::scene]", supers)
	}
	if supers := m.DirectSupertypes(nested(t, p.Scope, "focus", "picture")); len(supers) != 1 ||
		supers[0] != nested(t, focusDef.Scope, "image") {
		t.Fatalf("DirectSupertypes(focus::picture) = %v, want [Focus::image]", supers)
	}
}

// TestImplicitParameterRedefinitionNegativeCases covers the boundaries of the
// rule: an explicit redefinition governs, a direction mismatch is not a
// redefinition, a position the general behavior does not have is inherited
// rather than redefined, and an undirected feature is not a parameter at all.
func TestImplicitParameterRedefinitionNegativeCases(t *testing.T) {
	cases := []struct {
		name string
		src  string
		path []string
		want []string
	}{
		{
			name: "explicit redefinition governs",
			src: `package P {
				part def Image;
				action def Focus { in scene; out image : Image; }
				action focus : Focus { out item shot :>> Focus::image; in item scene; }
			}`,
			path: []string{"focus", "shot"},
			want: []string{"image"},
		},
		{
			name: "direction mismatch",
			src: `package P {
				action def Focus { in scene; out image; }
				action focus : Focus { out item image; }
			}`,
			path: []string{"focus", "image"},
			want: nil,
		},
		{
			name: "more parameters than the general behavior",
			src: `package P {
				action def Focus { in scene; }
				action focus : Focus { in item scene; in item lighting; }
			}`,
			path: []string{"focus", "lighting"},
			want: nil,
		},
		{
			name: "undirected feature is not a parameter",
			src: `package P {
				action def Focus { in scene; out image; }
				action focus : Focus { item image; }
			}`,
			path: []string{"focus", "image"},
			want: nil,
		},
		{
			name: "owner is not a behavior",
			src: `package P {
				part def Camera { in item image; }
				part camera : Camera { in item shot; }
			}`,
			path: []string{"camera", "shot"},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, root := buildModel(t, tc.src)
			p := sym(t, root, "P")
			supers := m.DirectSupertypes(nested(t, p.Scope, tc.path...))
			if len(supers) != len(tc.want) {
				t.Fatalf("DirectSupertypes(%v) = %v, want %v", tc.path, supers, tc.want)
			}
			for i, sup := range supers {
				if sup.Name != tc.want[i] {
					t.Fatalf("DirectSupertypes(%v)[%d] = %q, want %q", tc.path, i, sup.Name, tc.want[i])
				}
			}
		})
	}
}

// TestImplicitResultParameterRedefinition covers SysML v2 7.19.2: a result
// parameter redefines the result parameter of the general calculation whatever
// its position, and does not consume a position for the other parameters.
func TestImplicitResultParameterRedefinition(t *testing.T) {
	m, root := buildModel(t, `package P {
		attribute def Speed;
		calc def Average { in samples; return average : Speed; }
		calc average : Average { in samples; return result; }
	}`)
	p := sym(t, root, "P")
	averageDef := nested(t, p.Scope, "Average")

	if supers := m.DirectSupertypes(nested(t, p.Scope, "average", "samples")); len(supers) != 1 ||
		supers[0] != nested(t, averageDef.Scope, "samples") {
		t.Fatalf("DirectSupertypes(average::samples) = %v, want [Average::samples]", supers)
	}
	if supers := m.DirectSupertypes(nested(t, p.Scope, "average", "result")); len(supers) != 1 ||
		supers[0] != nested(t, averageDef.Scope, "average") {
		t.Fatalf("DirectSupertypes(average::result) = %v, want [Average::average]", supers)
	}
}

// TestImplicitParameterRedefinitionOfEveryGeneralBehavior covers a step with
// more than one general behavior: the parameter redefines the parameter at its
// position in each of them.
func TestImplicitParameterRedefinitionOfEveryGeneralBehavior(t *testing.T) {
	m, root := buildModel(t, `package P {
		action def A { in a1; }
		action def B { in b1; }
		action c :> A, B { in item c1; }
	}`)
	p := sym(t, root, "P")
	supers := m.DirectSupertypes(nested(t, p.Scope, "c", "c1"))
	if len(supers) != 2 || supers[0].Name != "a1" || supers[1].Name != "b1" {
		t.Fatalf("DirectSupertypes(c::c1) = %v, want [A::a1 B::b1]", supers)
	}
}

// TestInteractionParametersRedefineByPosition covers that an interaction is a
// behavior: its directed features are parameters, and a step typed by it
// redefines them by position while its ends are not parameters at all.
func TestInteractionParametersRedefineByPosition(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package P {
		class T; class U;
		interaction I { end a : T; end b : T; in x : T; out y : U; }
		interaction J specializes I { in p; out q; }
		step s : J { in r; out w; }
	}`)
	p := sym(t, root, "P")
	i := nested(t, p.Scope, "I")
	if supers := m.DirectSupertypes(nested(t, p.Scope, "J", "p")); len(supers) != 1 || supers[0] != nested(t, i.Scope, "x") {
		t.Fatalf("DirectSupertypes(J::p) = %v, want [I::x]", supers)
	}
	if supers := m.DirectSupertypes(nested(t, p.Scope, "s", "w")); len(supers) != 1 || supers[0] != nested(t, p.Scope, "J", "q") {
		t.Fatalf("DirectSupertypes(s::w) = %v, want [J::q]", supers)
	}
	if !m.Conforms(nested(t, p.Scope, "s", "r"), nested(t, p.Scope, "T")) {
		t.Fatalf("s::r does not take T through I::x")
	}
	if !m.Performable(PerformsBehavior, i) || !m.Performable(PerformsBehavior, nested(t, p.Scope, "s")) {
		t.Fatalf("interaction and step typed by it are not performable")
	}
}

// TestImplicitRedefinitionOfInheritedParameter covers the parameters a general
// behavior inherits rather than owns: they still occupy their position, so a
// parameter of a step typed by that behavior redefines the inherited one
// (KerML 7.4.7.2, "inheriting any additional parameters from the
// superclassifier").
func TestImplicitRedefinitionOfInheritedParameter(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Image;
		action def Focus { in scene; out image : Image; }
		action def AutoFocus :> Focus;
		action focus : AutoFocus { in item scene; out item image; }
	}`)
	p := sym(t, root, "P")
	want := nested(t, nested(t, p.Scope, "Focus").Scope, "image")
	if supers := m.DirectSupertypes(nested(t, p.Scope, "focus", "image")); len(supers) != 1 || supers[0] != want {
		t.Fatalf("DirectSupertypes(focus::image) = %v, want [Focus::image]", supers)
	}
}

// TestChainRedefinitionTargetStartsInTheGenerals covers that a redefinition
// target written as a chain (`:>> w.x`) starts at a feature the owning type
// inherits or the enclosing namespace offers, never at a sibling, whether the
// model or the document walk reads it first.
func TestChainRedefinitionTargetStartsInTheGenerals(t *testing.T) {
	src := `package P {
		part def W { attribute x; }
		part def A { part w : W; }
		part def B :> A { attribute z :>> w.x; }
		part def C { part w : W; attribute z :>> w.x; }
	}`
	for _, resolvedFirst := range []bool{false, true} {
		m, root, r, doc := buildUnresolvedModel(t, "t.sysml", source.KindSysML, src)
		if resolvedFirst {
			r.ResolveDocument("t.sysml", doc)
		}
		p := sym(t, root, "P")
		wantX := nested(t, p.Scope, "W", "x")
		if supers := m.DirectSupertypes(nested(t, p.Scope, "B", "z")); len(supers) != 1 || supers[0] != wantX {
			t.Errorf("resolvedFirst=%v: DirectSupertypes(B::z) = %v, want [W::x]", resolvedFirst, supers)
		}
		if supers := m.DirectSupertypes(nested(t, p.Scope, "C", "z")); len(supers) != 0 {
			t.Errorf("resolvedFirst=%v: DirectSupertypes(C::z) = %v, want none", resolvedFirst, supers)
		}
		if !resolvedFirst {
			r.ResolveDocument("t.sysml", doc)
		}
		if got := len(r.Diagnostics); got != 1 {
			t.Errorf("resolvedFirst=%v: diagnostics = %v, want one unresolved w", resolvedFirst, r.Diagnostics)
		}
	}
}

// TestInheritedParametersExcludeExplicitlyRedefinedOnes covers that a general
// behavior's parameters are inherited by what an owned parameter actually
// redefines, not by count: `shot :>> image` claims `image`, so the parameter
// left to inherit is `scene`, whatever position it was declared at.
func TestInheritedParametersExcludeExplicitlyRedefinedOnes(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Scene;
		part def Image;
		action def Focus { in scene : Scene; out image : Image; }
		action def AutoFocus :> Focus { out shot :>> image; }
		action af : AutoFocus { out item pic; in item s; }
	}`)
	p := sym(t, root, "P")
	autoFocus := nested(t, p.Scope, "AutoFocus")
	wantShot := nested(t, autoFocus.Scope, "shot")
	wantScene := nested(t, nested(t, p.Scope, "Focus").Scope, "scene")

	// AutoFocus's parameters are (shot, scene): position 0 is the owned `shot`,
	// and `image` is not inherited because `shot` redefines it.
	if supers := m.DirectSupertypes(nested(t, p.Scope, "af", "pic")); len(supers) != 1 || supers[0] != wantShot {
		t.Fatalf("DirectSupertypes(af::pic) = %v, want [AutoFocus::shot]", supers)
	}
	if supers := m.DirectSupertypes(nested(t, p.Scope, "af", "s")); len(supers) != 1 || supers[0] != wantScene {
		t.Fatalf("DirectSupertypes(af::s) = %v, want [Focus::scene]", supers)
	}
}

// TestInheritedParameterSurvivesADirectionMismatch covers that a position whose
// directions disagree is not a redefinition and does not consume the general
// parameter either: `out y` neither redefines `in a` nor hides it.
func TestInheritedParameterSurvivesADirectionMismatch(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Scene;
		action def Take { in scene : Scene; }
		action def Record :> Take { out y; }
		action r : Record { out item o; in item s; }
	}`)
	p := sym(t, root, "P")
	wantY := nested(t, nested(t, p.Scope, "Record").Scope, "y")
	wantScene := nested(t, nested(t, p.Scope, "Take").Scope, "scene")

	// Record's parameters are (y, scene): `out y` does not redefine `in scene`.
	if supers := m.DirectSupertypes(nested(t, p.Scope, "r", "o")); len(supers) != 1 || supers[0] != wantY {
		t.Fatalf("DirectSupertypes(r::o) = %v, want [Record::y]", supers)
	}
	if supers := m.DirectSupertypes(nested(t, p.Scope, "r", "s")); len(supers) != 1 || supers[0] != wantScene {
		t.Fatalf("DirectSupertypes(r::s) = %v, want [Take::scene]", supers)
	}
}
