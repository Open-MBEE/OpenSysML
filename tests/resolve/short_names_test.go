package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A feature with no declared name or short name takes the short name of the
// feature it redefines (KerML 8.3.3.3 Feature::effectiveShortName), so the
// redefining feature answers to that short name from the redefining type and
// from whatever specializes, subsets or redefines it, hiding the redefined
// feature's membership (8.3.3.1 Type::inheritedMemberships).
func TestEffectiveShortNameOfARedefiningFeatureResolves(t *testing.T) {
	const model = `package P {
		part def TMS { attribute mass; }
		part def ADS { part <tms> t : TMS; }
		part gear : ADS { part :>> t { } }
		part g2 :> gear;
		part g2b :> g2 { part :>> t; }
		abstract part <ads> adsFull { part <tmsU> thermalManagementSystem : TMS; }
		part gearU subsets ads { part :>> thermalManagementSystem { } }
	`
	cases := []struct {
		name, body, ref, binds string
	}{
		{"redefining type", `part c { attribute a :> gear.tms.mass; }`, "tms", "P::gear::t"},
		{"one level of specialization", `part c { attribute a :> g2.tms.mass; }`, "tms", "P::gear::t"},
		{"two levels of specialization", `part c :> g2 { attribute a :> c.tms.mass; }`, "tms", "P::gear::t"},
		{"redefined again two levels down", `part c { attribute a :> g2b.tms.mass; }`, "tms", "P::g2b::t"},
		{"through redefines", `part c { ref part sua redefines gear; attribute a :> sua.tms.mass; }`, "tms", "P::gear::t"},
		{"through subsets", `part c { ref part sua :> gear; attribute a :> sua.tms.mass; }`, "tms", "P::gear::t"},
		{"redefined feature on a usage", `part c { ref part sua :> gearU; attribute a :> sua.tmsU.mass; }`, "tmsU", "P::gearU::thermalManagementSystem"},
		{"qualified name", `attribute q :> gear::tms;`, "gear::tms", "P::gear::t"},
		{"qualified name one level down", `attribute q :> g2::tms;`, "g2::tms", "P::gear::t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, root, scope := resolvedDoc(t, model+tc.body+"\n}")
			if got := bindingOf(t, r, root, scope, tc.ref); got != tc.binds {
				t.Errorf("%s binds %q, want %q", tc.ref, got, tc.binds)
			}
			if _, errs := diagnosticsOf(r); len(errs) != 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}

// An import surfaces a membership under its short name (KerML 8.3.2.4
// Membership::memberShortName), the derived one included: a namespace import,
// a membership import naming the short name and a recursive import all bring
// the redefining feature in as `tms`.
func TestImportsSurfaceTheDerivedShortName(t *testing.T) {
	const model = `package Q {
		part def A { part <tms> t; }
		part def B :> A { part :>> t; }
	`
	cases := []struct{ name, body string }{
		{"namespace import", `package Use { private import B::*; part x :> tms; }`},
		{"membership import", `package Use { private import B::tms; part x :> tms; }`},
		{"recursive import", `package Use { private import B::**; part x :> tms; }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, root, scope := resolvedDoc(t, model+tc.body+"\n}")
			if got := bindingOf(t, r, root, scope, "tms"); got != "Q::B::t" {
				t.Errorf("tms binds %q, want Q::B::t", got)
			}
			if _, errs := diagnosticsOf(r); len(errs) != 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}

// Two imports bringing distinct features in under one derived short name hide
// both memberships (KerML 7.2.5.4), as they do under a declared one.
func TestCollidingDerivedShortNamesAreHidden(t *testing.T) {
	r, root, scope := resolvedDoc(t, `package Q {
		part def A { part <tms> t; }
		part def A2 :> A { part :>> t; }
		part def B { part <tms> u; }
		part def B2 :> B { part :>> u; }
		package Use { private import A2::*; private import B2::*; part x :> tms; }
	}`)
	if got := bindingOf(t, r, root, scope, "tms"); got != "" {
		t.Errorf("tms binds %q, want it hidden", got)
	}
	warnings, errs := diagnosticsOf(r)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Duplicate of imported member name 'tms'") {
		t.Errorf("warnings %v, want the duplicate imported member name", warnings)
	}
	if len(errs) != 1 || !strings.Contains(errs[0], "hidden here") {
		t.Errorf("errors %v, want the hidden-name diagnostic", errs)
	}
}

// A redefining feature that declares a short name of its own has that short
// name, not the redefined feature's (effectiveShortName returns the
// declaredShortName when one is given), and the redefinition hides the
// redefined feature, so the old short name no longer resolves.
func TestDeclaredShortNameOfARedefiningFeatureReplacesTheInheritedOne(t *testing.T) {
	const model = `package P {
		part def TMS { attribute mass; }
		part def ADS { part <tms> t : TMS; }
		part gear : ADS { part :>> t { } }
		part g3 :> gear { part <t3> :>> t; }
	`
	cases := []struct {
		name, body, ref, binds, unresolved string
	}{
		{"new short name", `part c { attribute a :> g3.t3.mass; }`, "t3", "P::g3::t3", ""},
		{"old short name", `part c { attribute a :> g3.tms.mass; }`, "tms", "", "unresolved member: tms"},
		{"old short name qualified", `attribute q :> g3::tms;`, "g3::tms", "", "unresolved reference: g3::tms"},
		{"old short name one level down", `part g4 :> g3; part c { attribute a :> g4.tms.mass; }`, "tms", "", "unresolved member: tms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, root, scope := resolvedDoc(t, model+tc.body+"\n}")
			if got := bindingOf(t, r, root, scope, tc.ref); got != tc.binds {
				t.Errorf("%s binds %q, want %q", tc.ref, got, tc.binds)
			}
			_, errs := diagnosticsOf(r)
			if tc.unresolved == "" {
				if len(errs) != 0 {
					t.Errorf("unexpected errors: %v", errs)
				}
				return
			}
			if len(errs) != 1 || !strings.Contains(errs[0], tc.unresolved) {
				t.Errorf("errors %v, want one containing %q", errs, tc.unresolved)
			}
		})
	}
}

// A derived short name may be asked for while the owner's own header is being
// resolved from its body, before the owner's generals are decided: that early
// query binds nothing, and the redefinition still resolves among the generals
// once they are, not to a like-named element outside.
func TestEarlyShortNameQueryLeavesTheRedefinitionToItsGenerals(t *testing.T) {
	const name = "early.sysml"
	const src = `package P {
		private import Metaobjects::SemanticMetadata;
		part def <env> Environment;
		part def Base { part <e> env; }
		part bases : Base[*];
		metadata def <ctx> CtxMeta :> SemanticMetadata { :>> baseType = bases meta SysML::Usage; }
		#ctx part x { part :>> env : Environment; }
	}`
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	r.ResolveDocument(name, root)
	if got := bindingOf(t, r, root, idx.DocumentRoot(name), "env"); got != "P::Base::env" {
		t.Errorf("env binds %q, want P::Base::env", got)
	}
	if _, errs := diagnosticsOf(r); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}
