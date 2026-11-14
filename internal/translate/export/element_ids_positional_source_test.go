package export

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A transition's positional source — the member before a state body's `then x;`, a guarded
// `transition if g then x;` or a decision's branch — has its Membership's id without a conversion.
func TestElementIDsRecordThePositionalSourceMembershipWithoutConverting(t *testing.T) {
	sf := source.New("positional.sysml", []byte(`package P {
    attribute g : ScalarValues::Boolean;
    state def S {
        state idle;
        then busy;
        state busy;
        transition if g then idle;
    }
    action def A {
        action a1;
        decide d;
        if g then a2;
        else a3;
        action a2;
        action a3;
    }
}`))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	docs := []ModelDocument{{File: sf, Root: root}}
	graph, _, encoders, err := modelToRDF(docs, IDQualifiedName)
	if err != nil {
		t.Fatalf("modelToRDF: %v", err)
	}
	ids, err := NewElementIDs(docs)
	if err != nil {
		t.Fatalf("NewElementIDs: %v", err)
	}
	var recorded []string
	for node := range encoders[0].names.FQN {
		switch node.(type) {
		case *ast.TransitionMember, *ast.SuccessionEdge, *ast.ControlFlowEdge:
		default:
			continue
		}
		if _, membership, ok := ids.OfExpressionPart(sf.Name(), node, "psourcemember"); ok {
			recorded = append(recorded, membership)
		}
	}
	var written []string
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value == rdf.SysML+pElementID && strings.HasSuffix(triple.Object.Value, "_psourcemember") {
			written = append(written, triple.Object.Value)
		}
	}
	sort.Strings(recorded)
	sort.Strings(written)
	if len(written) != 4 {
		t.Fatalf("written source memberships = %q, want one per transition", written)
	}
	if strings.Join(recorded, " ") != strings.Join(written, " ") {
		t.Fatalf("recorded source memberships = %q, written %q", recorded, written)
	}
}
