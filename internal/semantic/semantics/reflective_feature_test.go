package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

var reflectiveUsageFeatureFlags = []string{
	"isEnd", "isPortion", "isConstant", "isVariable", "isComposite",
	"isDerived", "isAbstract", "isOrdered", "isUnique", "isReference",
}

func assertReflectiveFeatureFlags(t *testing.T, m *Model, sym *symbols.Symbol) {
	t.Helper()
	for _, feature := range reflectiveUsageFeatureFlags {
		if _, ok := m.ReflectiveFeatureValue(sym, feature); !ok {
			t.Errorf("%s.%s is underived", symbols.FQNOf(sym), feature)
		}
	}
}

func TestReflectiveAttributeFeaturesAreReferentialAndNonComposite(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		attribute def A { part definitionFeature; }
		attribute a : A { part usageFeature; }
	}`)
	for _, path := range []string{"P::A::definitionFeature", "P::a::usageFeature"} {
		feature := nestedSym(t, root, path)
		for name, want := range map[string]bool{"isComposite": false, "isReference": true} {
			got, ok := m.ReflectiveFeatureValue(feature, name)
			if !ok || got.Bool != want {
				t.Errorf("%s.%s = %v (present %t), want %t", path, name, got, ok, want)
			}
		}
	}
}

func TestReflectiveFeatureFlagsForWrapperDeclarations(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		part def Owner { part child; ref part borrowed; }
		action def Flow {
			fork f; join j; merge m; decide d;
			first start;
		}
		requirement def Req {
			subject subject : Owner;
			assume constraint { true; }
			require constraint { true; }
		}
		metadata def Tag;
		action def Marked { @ annotation : Tag; }
		state machine {
			state running { state idle; }
			transition t first running accept after 1 [SI::s] then running;
		}
	}`)
	paths := []string{
		"P::Owner::child", "P::Owner::borrowed",
		"P::Flow::f", "P::Flow::j", "P::Flow::m", "P::Flow::d",
		"P::Flow::start",
		"P::Req::subject",
		"P::Marked::annotation",
		"P::machine::running", "P::machine::running::idle", "P::machine::t",
	}
	for _, path := range paths {
		feature := nestedSym(t, root, path)
		assertReflectiveFeatureFlags(t, m, feature)
	}
	for _, path := range []string{"P::Flow::f", "P::Flow::j", "P::Flow::m", "P::Flow::d"} {
		got, ok := m.ReflectiveFeatureValue(nestedSym(t, root, path), "isComposite")
		if !ok || !got.Bool {
			t.Errorf("%s.isComposite = %v (present %t), want true", path, got, ok)
		}
	}
}

func TestReflectiveFeatureFlagsOnRecordedUsagesAndControlNodes(t *testing.T) {
	src := symbols.NewIndex()
	addTestDoc(t, src, "lib.sysml", `part def Owner {
		part child;
		ref part borrowed;
		constant attribute frozen;
	}
	action def Flow { fork f; }`)
	record, err := symbols.RecordScope(
		src.DocumentRoot("lib.sysml"),
		func(*symbols.Symbol) bool { return true },
		func(sym *symbols.Symbol) symbols.LibraryFacts {
			facts := symbols.LibraryFacts{
				Node:    symbols.NodeKindOf(sym.Decl),
				Keyword: sym.Keyword(),
			}
			if usage, ok := sym.Decl.(*ast.Usage); ok {
				facts.UsageKind = usage.Kind
				facts.Direction = usage.Direction
				if usage.IsEnd {
					facts.Modifiers |= symbols.ModEnd
				}
				if usage.IsPortion || usage.Portion != ast.PortionNone {
					facts.Modifiers |= symbols.ModPortion
				}
				if usage.IsConstant {
					facts.Modifiers |= symbols.ModConstant
				}
				if usage.IsVariable {
					facts.Modifiers |= symbols.ModVariable
				}
				if usage.IsReference {
					facts.Modifiers |= symbols.ModReference
				}
				if usage.IsComposite {
					facts.Modifiers |= symbols.ModComposite
				}
				if usage.IsDerived {
					facts.Modifiers |= symbols.ModDerived
				}
				if usage.IsOrdered {
					facts.Modifiers |= symbols.ModOrdered
				}
				if usage.IsNonunique {
					facts.Modifiers |= symbols.ModNonunique
				}
			}
			return facts
		},
	)
	if err != nil {
		t.Fatalf("RecordScope: %v", err)
	}
	recorded, err := symbols.BuildRecorded(record, "lib.sysml")
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	idx := stdlibIndex(t)
	idx.AddRecordedDocument("lib.sysml", source.KindSysML, recorded, nil)
	res := resolve.New(idx)
	m := NewModel(res)
	res.SetModel(m)
	root := idx.DocumentRoot("lib.sysml")
	child := nestedSym(t, root, "Owner::child")
	borrowed := nestedSym(t, root, "Owner::borrowed")
	frozen := nestedSym(t, root, "Owner::frozen")
	fork := nestedSym(t, root, "Flow::f")
	if !child.Recorded() || !borrowed.Recorded() || !frozen.Recorded() || !fork.Recorded() {
		t.Fatal("recorded symbols lost their recorded status")
	}
	assertReflectiveFeatureFlags(t, m, child)
	assertReflectiveFeatureFlags(t, m, borrowed)
	assertReflectiveFeatureFlags(t, m, frozen)
	assertReflectiveFeatureFlags(t, m, fork)
	for name, want := range map[string]bool{"isComposite": false, "isReference": true} {
		got, ok := m.ReflectiveFeatureValue(borrowed, name)
		if !ok || got.Bool != want {
			t.Errorf("recorded borrowed.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
	for name, want := range map[string]bool{"isConstant": true, "isVariable": false} {
		got, ok := m.ReflectiveFeatureValue(frozen, name)
		if !ok || got.Bool != want {
			t.Errorf("recorded frozen.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
	got, ok := m.ReflectiveFeatureValue(fork, "isComposite")
	if !ok || !got.Bool {
		t.Errorf("recorded fork.isComposite = %v (present %t), want true", got, ok)
	}
}

func TestReflectivePortUsageNestedFeaturesAndConnectorEnds(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		port def PD;
		part host {
			port p : PD { part nested; }
			interface i connect (e ::> p);
		}
	}`)
	port := nestedSym(t, root, "P::host::p")
	nested := nestedSym(t, root, "P::host::p::nested")
	assertReflectiveFeatureFlags(t, m, port)
	for name, want := range map[string]bool{"isComposite": true, "isReference": false} {
		got, ok := m.ReflectiveFeatureValue(nested, name)
		if !ok || got.Bool != want {
			t.Errorf("nested %s = %v (present %t), want %t", name, got, ok, want)
		}
	}

	connectorEnd := nestedSym(t, root, "P::host::i::e")
	if _, ok := connectorEnd.Decl.(*ast.ConnectorEnd); !ok {
		t.Fatalf("connector end declaration = %T, want *ast.ConnectorEnd", connectorEnd.Decl)
	}
	assertReflectiveFeatureFlags(t, m, connectorEnd)
	if got := m.MetaclassOf(connectorEnd); got == nil || got.Name != "PortUsage" {
		t.Errorf("connector end metaclass = %v, want SysML::PortUsage", got)
	}
	nestedUsages, ok := m.reflectiveOwnedUsages(connectorEnd, "nestedUsage")
	if !ok || len(nestedUsages) != 0 {
		t.Errorf("connector-end nestedUsage = %v, %v; want a derived empty sequence", fqns(nestedUsages), ok)
	}
}
