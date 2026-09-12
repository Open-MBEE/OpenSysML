package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A redefinition owned by a feature of a type names a feature of that type's
// generals; failing that, it is looked up from the enclosing namespace outward.
// The owning type's own members, imports and aliases are never candidates
// (KerML 8.2.3.5.2). Each verdict below is the pilot implementation's.
var redefinitionTargetCases = []struct {
	name string
	src  string
	// want maps each reference as written to the element it names, or to ""
	// when it must stay unresolved.
	want map[string]string
}{
	{
		name: "a sibling is not a target",
		src: `package P {
			part def B {
				attribute x;
				attribute z :>> x;
			}
		}`,
		want: map[string]string{"x": ""},
	},
	{
		name: "an import of the owning type is not consulted",
		src: `package Lib { attribute x; }
		package P {
			part def B {
				private import Lib::*;
				attribute z :>> x;
			}
		}`,
		want: map[string]string{"x": ""},
	},
	{
		name: "an alias of the owning type is not consulted",
		src: `package P {
			part def A { attribute x; }
			part def B :> A {
				alias y for A::x;
				attribute z :>> y;
			}
		}`,
		want: map[string]string{"y": ""},
	},
	{
		name: "a qualified name does not start at a sibling",
		src: `package P {
			part def C { attribute c1; }
			part def D :> C {
				part C { attribute nope; }
				attribute d :>> C::nope;
			}
		}`,
		want: map[string]string{"C::nope": ""},
	},
	{
		name: "the enclosing namespace is searched past the owning type",
		src: `package P {
			attribute x;
			part def B {
				attribute x;
				attribute z :>> x;
			}
		}`,
		want: map[string]string{"x": "P::x"},
	},
	{
		name: "an import of the enclosing namespace is searched",
		src: `package Lib { attribute x; }
		package P {
			private import Lib::*;
			part def B {
				attribute x;
				attribute z :>> x;
			}
		}`,
		want: map[string]string{"x": "Lib::x"},
	},
	{
		name: "an alias of the enclosing namespace is searched",
		src: `package P {
			attribute x;
			alias y for x;
			part def B {
				attribute y;
				attribute z :>> y;
			}
		}`,
		want: map[string]string{"y": "P::x"},
	},
	{
		name: "a qualified name starts in the enclosing namespace",
		src: `package P {
			part def C { attribute c1; attribute nope; }
			part def D :> C {
				part C { attribute nope; }
				attribute d :>> C::nope;
			}
		}`,
		want: map[string]string{"C::nope": "P::C::nope"},
	},
	{
		name: "a first segment a general offers shadows the enclosing namespace",
		src: `package P {
			part def A { part q; }
			part def C;
			part def G { part faces : C; }
			part def Outer {
				part faces : A;
				part ff : G :> faces { part e :>> faces::q; }
			}
		}`,
		want: map[string]string{"faces::q": ""},
	},
	{
		name: "an inherited feature wins over a namesake sibling",
		src: `package P {
			part def A { attribute x; }
			part def B :> A {
				attribute x;
				attribute z :>> x;
			}
		}`,
		want: map[string]string{"x": "P::A::x"},
	},
	{
		name: "a chain starts at an inherited feature",
		src: `package P {
			part def W { attribute x; }
			part def A { part w : W; }
			part def B :> A {
				attribute z :>> w.x;
			}
		}`,
		want: map[string]string{"w": "P::A::w", "x": "P::W::x"},
	},
	{
		name: "a usage-owned redefinition reaches the usage's type",
		src: `package P {
			part def A { attribute x; }
			part def B {
				part a : A {
					attribute x;
					attribute z :>> x;
				}
			}
		}`,
		want: map[string]string{"x": "P::A::x"},
	},
	{
		name: "a usage-owned redefinition reaches what the usage subsets",
		src: `package P {
			part def A { attribute x; }
			part a : A;
			part def B {
				part b :> a {
					attribute z :>> x;
				}
			}
		}`,
		want: map[string]string{"x": "P::A::x"},
	},
	{
		name: "a public import of a general contributes its members",
		src: `package Lib { attribute a1; }
		package P {
			part def S { public import Lib::*; }
			part def T :> S {
				attribute b :>> a1;
			}
		}`,
		want: map[string]string{"a1": "Lib::a1"},
	},
	{
		name: "a redefined feature's type still offers a masked namesake",
		src: `package P {
			port def PwrCmdPort { in item pwrCmd; }
			port def FuelCmdPort { in item fuelCmd; }
			part def V { port pwrCmdPort : PwrCmdPort; }
			part def V2 :> V {
				port fuelCmdPort : FuelCmdPort :>> pwrCmdPort {
					in item fuelCmd :>> pwrCmd;
				}
			}
		}`,
		want: map[string]string{"pwrCmdPort": "P::V::pwrCmdPort", "pwrCmd": "P::PwrCmdPort::pwrCmd"},
	},
	{
		name: "a nested redefinition reaches the redefined feature's members",
		src: `package P {
			part def A { part x { attribute y; } }
			part def B :> A {
				part :>> x { attribute :>> y; }
			}
		}`,
		want: map[string]string{"x": "P::A::x", "y": "P::A::x::y"},
	},
	{
		name: "a qualified name may name the owning type from the enclosing namespace",
		src: `package P {
			part def B {
				attribute x;
				attribute z :>> B::x;
			}
			part def A { attribute y; }
			part def C :> A {
				attribute w :>> C::y;
			}
		}`,
		want: map[string]string{"B::x": "P::B::x", "C::y": "P::A::y"},
	},
	{
		name: "a chain through the enclosing namespace reaches the redefining feature's inherited members",
		src: `package P {
			part def Disc { item edges; }
			part def Cyl { item af : Disc { item :>> Disc::edges; } }
			part def CircCyl :> Cyl {
				item :>> af : Disc { ref :>> af::edges, Disc::edges; }
			}
		}`,
		want: map[string]string{"af::edges": "P::Cyl::af::edges", "Disc::edges": "P::Disc::edges"},
	},
	{
		name: "a chain through the enclosing namespace passes a bodiless redefinition on the way",
		src: `package P {
			part def Disc { item edges; }
			part def Shell { item faces; item edges; }
			part def Cyl :> Shell { item af : Disc [0..1] :> faces { item :>> Disc::edges; } }
			part def Cyl2 :> Cyl { item :>> af [1]; }
			part def CircCyl :> Cyl2 {
				item :>> af : Disc { ref :>> af::edges, Disc::edges; }
			}
		}`,
		want: map[string]string{"af::edges": "P::Cyl::af::edges", "Disc::edges": "P::Disc::edges"},
	},
	{
		name: "an alias a general owns names what it aliases",
		src: `package P {
			part def A { port porig; alias po for porig; }
			part a : A { port po :>> po; }
		}`,
		want: map[string]string{"po": "P::A::porig"},
	},
	{
		name: "a cycle among generals does not make a feature its own target",
		src: `package P {
			part def T;
			part def M :> T, F;
			part def F :> M { part t :>> M::t; }
		}`,
		want: map[string]string{"M::t": ""},
	},
	{
		name: "a cycle among generals still reaches the inherited feature",
		src: `package P {
			part def T { part t; }
			part def M :> T, F;
			part def F :> M { part t :>> M::t; }
		}`,
		want: map[string]string{"M::t": "P::T::t"},
	},
	{
		name: "a cycle among generals does not make a simple name its own target",
		src: `package P {
			part def M :> F;
			part def F :> M { part t :>> t; }
		}`,
		want: map[string]string{"t": ""},
	},
	{
		name: "a cycle among generals passes a simple name on to the inherited feature",
		src: `package P {
			part def T { part t; }
			part def M :> F, T;
			part def F :> M { part t :>> t; }
		}`,
		want: map[string]string{"t": "P::T::t"},
	},
	{
		name: "a cycle among generals reaches a sibling as an inherited feature",
		src: `package P {
			part def M :> F;
			part def F :> M { part t; part u :>> t; }
		}`,
		want: map[string]string{"t": "P::F::t"},
	},
	{
		name: "a package-owned redefinition resolves as an ordinary name",
		src: `package P {
			attribute x;
			attribute z :>> x;
		}`,
		want: map[string]string{"x": "P::x"},
	},
}

func TestRedefinitionTargetsFollowTheGeneralsThenTheEnclosingNamespace(t *testing.T) {
	for _, tc := range redefinitionTargetCases {
		for _, withModel := range []bool{false, true} {
			name := tc.name + " (relationship walk)"
			if withModel {
				name = tc.name + " (semantic model)"
			}
			t.Run(name, func(t *testing.T) {
				r, root, rootScope := redefinitionDoc(t, tc.src, withModel)
				checkRedefinitionVerdicts(t, r, root, rootScope, tc.want)
			})
		}
	}
}

// redefinitionDoc resolves src with or without a semantic model attached: the
// generals of an owning type are read from the model when there is one and
// from its declared relationships otherwise.
func redefinitionDoc(t *testing.T, src string, withModel bool) (*resolve.Resolver, *ast.RootNamespace, *symbols.Scope) {
	t.Helper()
	p := parser.New(source.New("app.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("app.sysml", root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	if withModel {
		r.SetModel(semantics.NewModel(r))
	}
	r.ResolveDocument("app.sysml", root)
	return r, root, idx.DocumentRoot("app.sysml")
}

// checkRedefinitionVerdicts checks every reference listed in want against what
// the document walk bound and what resolving it on its own yields, and that
// the unresolved ones are exactly the diagnostics reported.
func checkRedefinitionVerdicts(t *testing.T, r *resolve.Resolver, root *ast.RootNamespace, rootScope *symbols.Scope, want map[string]string) {
	t.Helper()
	seen := map[string]bool{}
	for _, ref := range resolve.References(root, rootScope) {
		text := nameText(ref.QN)
		wantFQN, listed := want[text]
		if !listed {
			continue
		}
		seen[text] = true
		sym, ok := r.ResolveReference(ref)
		switch {
		case wantFQN == "" && ok:
			t.Errorf("%s resolved to %s, want unresolved", text, symbols.FQNOf(sym))
		case wantFQN != "" && !ok:
			t.Errorf("%s is unresolved, want %s", text, wantFQN)
		case wantFQN != "" && symbols.FQNOf(sym) != wantFQN:
			t.Errorf("%s resolved to %s, want %s", text, symbols.FQNOf(sym), wantFQN)
		}
		if walked, walkedOK := r.PartSymbol(ref.QN, len(ref.QN.Parts)-1); walkedOK != ok || walked != sym {
			t.Errorf("%s: the document walk bound %v, resolving alone yields %v", text, walked, sym)
		}
	}
	var unresolved []string
	for text, fqn := range want {
		if !seen[text] {
			t.Errorf("%s was not collected as a reference", text)
		}
		if fqn == "" {
			unresolved = append(unresolved, text)
		}
	}
	if len(r.Diagnostics) != len(unresolved) {
		t.Errorf("diagnostics = %v, want exactly one for each of %v", r.Diagnostics, unresolved)
	}
	for _, d := range r.Diagnostics {
		named := false
		for _, text := range unresolved {
			if strings.HasPrefix(d.Message, "unresolved reference: "+text) {
				named = true
			}
		}
		if !named {
			t.Errorf("unexpected diagnostic %q", d.Message)
		}
	}
}
