package export

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// membershipModel writes a model of n parts, each with a comment and an attribute,
// so its graph holds about 3n memberships.
func membershipModel(n int) string {
	var b strings.Builder
	b.WriteString("// A model whose memberships outnumber its lines.\n\npackage Fleet {\n    part def Vehicle;\n\n")
	for i := range n {
		fmt.Fprintf(&b, "    // Vehicle %d.\n    part v%d : Vehicle {\n        attribute mass%d;\n    }\n\n", i, i, i)
	}
	b.WriteString("}\n")
	return b.String()
}

func graphOfNotation(tb testing.TB, notation string) *rdf.Graph {
	tb.Helper()
	file := source.New("n.sysml", []byte(notation))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		tb.Fatalf("fixture does not parse: %v", p.Diagnostics)
	}
	graph, err := ToRDF(file, root)
	if err != nil {
		tb.Fatal(err)
	}
	return graph
}

// withoutMembershipStatements drops the source, target, relatedElement, memberName
// and memberShortName statements of every Membership, as a writer before them
// left the graph, and returns how many it dropped.
func withoutMembershipStatements(graph *rdf.Graph) (*rdf.Graph, int) {
	stripped := rdf.NewGraphOf(append([]rdf.Triple(nil), graph.Triples()...), graph.Prefixes)
	dropped := 0
	stripped.RewriteTriples(func(t *rdf.Triple) bool {
		if !derivedMembershipOmitted(rdf.NewGraph(), stripped, *t) {
			return true
		}
		dropped++
		return false
	})
	return stripped, dropped
}

// A membership statement the candidate derives and the graph omits is not a
// disagreement; one the graph states differently is.
func TestDerivedMembershipStatementsAreNotDisagreements(t *testing.T) {
	full := graphOfNotation(t, membershipModel(3))
	stripped, dropped := withoutMembershipStatements(full)
	if dropped == 0 {
		t.Fatal("the writer states no membership ends to omit")
	}
	if n := len(disagreeingTriples(stripped, full)); n != 0 {
		t.Errorf("%d omitted membership statements counted as disagreements: %v", n, disagreeingTriples(stripped, full))
	}

	renamed := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
	changed := 0
	renamed.RewriteTriples(func(t *rdf.Triple) bool {
		if t.Predicate.Value == rdf.SysML+pMemberName && t.Object.Value == "v1" {
			t.Object = rdf.String("w1")
			changed++
		}
		return true
	})
	if changed != 1 {
		t.Fatalf("renamed %d memberNames, want 1", changed)
	}
	if len(disagreeingTriples(renamed, full)) == 0 {
		t.Error("a memberName the graph states against the candidate is not a disagreement")
	}
}

// holderIndex answers what a walk of the triples answers, for every node.
func TestHolderIndexMatchesTripleWalk(t *testing.T) {
	graph := graphOfNotation(t, membershipModel(3))
	walk := func(iri string) string {
		for _, t := range graph.Triples() {
			if t.Object.IsIRI() && t.Object.Value == iri && t.Subject.Value != iri {
				return t.Subject.Value
			}
		}
		return ""
	}
	var holders holderIndex
	checked := 0
	for _, triple := range graph.Triples() {
		if !triple.Object.IsIRI() {
			continue
		}
		if got, want := holders.pointingAt(graph, triple.Object.Value), walk(triple.Object.Value); got != want {
			t.Errorf("holder of <%s>: got <%s>, want <%s>", triple.Object.Value, got, want)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("the graph has no node to look up")
	}
	if got := holders.pointingAt(graph, "urn:nowhere"); got != "" {
		t.Errorf("holder of an unknown node: %q", got)
	}
	if !ontology.IsAncestorOrSelf("OwningMembership", mMembership) {
		t.Fatal("OwningMembership is not a Membership")
	}
}

// BenchmarkDecodePre1855Turtle decodes a graph written without membership
// statements at three sizes; ns/op should grow about linearly with n.
func BenchmarkDecodePre1855Turtle(b *testing.B) {
	for _, n := range []int{50, 100, 200} {
		stripped, dropped := withoutMembershipStatements(graphOfNotation(b, membershipModel(n)))
		if dropped == 0 {
			b.Fatal("the writer states no membership ends to omit")
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for b.Loop() {
				if _, err := ToSysML(stripped); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
