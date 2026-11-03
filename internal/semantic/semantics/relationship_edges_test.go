package semantics

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func buildRelationshipModelWithStdlib(t *testing.T, name string, kind source.Kind, src string) (*Model, *symbols.Scope) {
	t.Helper()
	idx := stdlibIndex(t)
	for _, libraryDoc := range idx.Documents() {
		idx.MarkLibrary(libraryDoc)
	}
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocumentWithKind(name, root, kind)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	return m, idx.DocumentRoot(name)
}

func nestedRelationshipUsage(t *testing.T, owner *symbols.Symbol, keyword string) *symbols.Symbol {
	t.Helper()
	var found *symbols.Symbol
	seen := map[*symbols.Scope]bool{}
	var visit func(*symbols.Scope)
	visit = func(scope *symbols.Scope) {
		if scope == nil || seen[scope] || found != nil {
			return
		}
		seen[scope] = true
		for _, member := range scope.AllMembers() {
			if usage, ok := member.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageSatisfy && usage.Keyword == keyword {
				found = member
				return
			}
			visit(member.Scope)
		}
	}
	visit(owner.Scope)
	if found == nil {
		t.Fatalf("%s has no nested %s relationship usage", owner.Name, keyword)
	}
	return found
}

func TestRelationshipEdgesOf(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		part def Component;
		part source : Component {
			port outPort;
			port inPort;
			connection link connect outPort to inPort;
		}
		part target : Component;
		allocation alloc allocate source to target;
		requirement req;
		part satisfiedBy { satisfy req; }
		requirement primary;
		requirement secondary;
		alias Secondary for secondary;
		part pump { satisfy requirement proof :> primary, Secondary; }
		verification def Check;
		verification verified : Check {
			objective { verify req; }
		}
	}`)
	index := m.resolver.Index()
	names := func(edges []RelationshipEdge) [][2]string {
		var out [][2]string
		for _, edge := range edges {
			out = append(out, [2]string{index.GetFQN(edge.Source), index.GetFQN(edge.Target)})
		}
		return out
	}
	cases := []struct {
		symbol  string
		keyword string
		kind    RelationshipKind
		want    [][2]string
	}{
		{"P::source::link", "", RelationshipConnection, [][2]string{{"P::source::outPort", "P::source::inPort"}}},
		{"P::alloc", "", RelationshipAllocation, [][2]string{{"P::source", "P::target"}}},
		{"P::satisfiedBy", "satisfy", RelationshipSatisfaction, [][2]string{{"P::satisfiedBy", "P::req"}}},
		{"P::verified", "verify", RelationshipVerification, [][2]string{{"P::verified", "P::req"}}},
		// A declaring satisfy relates its subject to the declared requirement and
		// to every requirement it subsets, an alias followed to its target.
		{"P::pump", "satisfy", RelationshipSatisfaction, [][2]string{{"P::pump", "P::pump::proof"}, {"P::pump", "P::primary"}, {"P::pump", "P::secondary"}}},
	}
	for _, tc := range cases {
		t.Run(tc.symbol+"/"+string(tc.kind), func(t *testing.T) {
			sym := nestedSym(t, root, tc.symbol)
			if tc.keyword != "" {
				sym = nestedRelationshipUsage(t, sym, tc.keyword)
			}
			got := names(m.RelationshipEdgesOf(sym, tc.kind))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RelationshipEdgesOf(%s, %s) = %v, want %v", tc.symbol, tc.kind, got, tc.want)
			}
		})
	}
	if got := m.RelationshipEdgesOf(nil, RelationshipConnection); len(got) != 0 {
		t.Fatalf("RelationshipEdgesOf(nil) = %v, want none", got)
	}
}

func TestRelationshipEdgesOfReturnsNoEdgesForUnsupportedKind(t *testing.T) {
	m, root := buildModelWithStdlib(t, "package P { part source; }")
	source, _ := root.LookupLocal("P")
	source, _ = source.Scope.LookupLocal("source")
	if got := m.RelationshipEdgesOf(source, RelationshipKind("unknown")); len(got) != 0 {
		t.Fatalf("RelationshipEdgesOf(unknown) = %v, want none", got)
	}
	if got := (*Model)(nil).RelationshipEdgesOf((*symbols.Symbol)(nil), RelationshipDependency); len(got) != 0 {
		t.Fatalf("nil model RelationshipEdgesOf = %v, want none", got)
	}
}

func TestDerivationEdgesKeepTaggedParticipants(t *testing.T) {
	m, root := buildRelationshipModelWithStdlib(t, "t.sysml", source.KindSysML, `package P {
		private import RequirementDerivation::*;
		requirement parent { @original; }
		requirement child { @derive; }
		requirement plainChild;
		#derivation connection byMetadata { end #original ::> parent; end #derive ::> child; }
		connection bySubsetting : Derivation {
			end :> originalRequirements ::> parent;
			end :> derivedRequirements ::> plainChild;
		}
		connection def TaggedDerivation :> Derivation {
			end #original src :> parent;
			end #derive dst :> child;
		}
	}`)
	index := m.resolver.Index()
	for symbol, want := range map[string][2]string{
		"P::byMetadata":       {"P::parent", "P::child"},
		"P::bySubsetting":     {"P::parent", "P::plainChild"},
		"P::TaggedDerivation": {"P::parent", "P::child"},
	} {
		edges := m.RelationshipEdgesOf(nestedSym(t, root, symbol), RelationshipDerivation)
		if len(edges) != 1 || index.GetFQN(edges[0].Source) != want[0] || index.GetFQN(edges[0].Target) != want[1] {
			t.Fatalf("RelationshipEdgesOf(%s) = %v, want %v -> %v", symbol, edges, want[0], want[1])
		}
	}
}

func TestRelationshipEdgesOfDerivationRefinementAndDependency(t *testing.T) {
	m, root := buildRelationshipModelWithStdlib(t, "t.sysml", source.KindSysML, `package P {
		private import RequirementDerivation::*;
		private import ModelingMetadata::Refinement;
		requirement originalRequirement;
		requirement derivedRequirement;
		part client;
		connection deriveLink : Derivation connect originalRequirement to derivedRequirement;
		dependency plain from client to originalRequirement;
		dependency refined from client to derivedRequirement { @Refinement; }
	}`)
	index := m.resolver.Index()
	names := func(edges []RelationshipEdge) [][2]string {
		var out [][2]string
		for _, edge := range edges {
			out = append(out, [2]string{index.GetFQN(edge.Source), index.GetFQN(edge.Target)})
		}
		return out
	}
	cases := []struct {
		symbol string
		kind   RelationshipKind
		want   [][2]string
	}{
		{"P::deriveLink", RelationshipDerivation, [][2]string{{"P::originalRequirement", "P::derivedRequirement"}}},
		{"P::plain", RelationshipDependency, [][2]string{{"P::client", "P::originalRequirement"}}},
		{"P::refined", RelationshipRefinement, [][2]string{{"P::client", "P::derivedRequirement"}}},
		{"P::refined", RelationshipDependency, [][2]string{{"P::client", "P::derivedRequirement"}}},
	}
	for _, tc := range cases {
		t.Run(tc.symbol+"/"+string(tc.kind), func(t *testing.T) {
			sym := nestedSym(t, root, tc.symbol)
			got := names(m.RelationshipEdgesOf(sym, tc.kind))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("RelationshipEdgesOf(%s, %s) = %v, want %v", tc.symbol, tc.kind, got, tc.want)
			}
		})
	}
}
