package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An indexed end attaches to the whole feature — what it reference-subsets and
// resolves as — and carries the index beside it; its multiplicity is one.
func TestIndexedConnectorEndAttachesTheFeatureAndKeepsTheIndex(t *testing.T) {
	m, root := buildModel(t, `package F3Idx {
		port def P;
		part def Source { port y : P[2]; }
		part def Sink { port u : P; }
		connection def C { end source[1] : P; end target[1] : P; }
		part def Asm {
			part s : Source;
			part k : Sink;
			connection c : C connect [1] s.y#(1) to [1] k.u;
			flow f from s.y#(2) to k.u;
			binding b bind s.y#(1) = k.u;
		}
	}`)
	asm := nested(t, sym(t, root, "F3Idx").Scope, "Asm")
	for _, name := range []string{"c", "f", "b"} {
		conn := nested(t, asm.Scope, name)
		ends := m.ConnectorObjectEnds(conn)
		if len(ends) != 2 {
			t.Fatalf("%s: %d ends, want two", name, len(ends))
		}
		chain, ok := ends[0].Attachment.(*ast.FeatureChainExpr)
		if !ok || ast.SimpleName(chain.Member) != "y" {
			t.Errorf("%s: end 0 attaches %T, want the chain s.y", name, ends[0].Attachment)
		}
		if ends[0].Index == nil || ends[0].Selection == nil {
			t.Errorf("%s: end 0 carries no index (%v) or selection (%v)", name, ends[0].Index, ends[0].Selection)
		}
		if _, ok := ends[0].Selection.(*ast.IndexExpr); !ok {
			t.Errorf("%s: end 0 selection is %T, want the IndexExpr as written", name, ends[0].Selection)
		}
		if ends[1].Index != nil || ends[1].Selection != nil {
			t.Errorf("%s: end 1 is not indexed, got index %v", name, ends[1].Index)
		}
		paths := m.ConnectorEndPaths(conn)
		if len(paths) != 2 || len(paths[0].Features) != 2 || paths[0].Features[1].Name != "y" {
			t.Errorf("%s: end paths = %+v, want s.y resolved through the chain", name, paths)
		}
	}
	if !m.ConnectorEndMultiplicityIsOne(nested(t, asm.Scope, "c"), 0) {
		t.Error("an indexed end selects one element, so its multiplicity is one")
	}
	flows := m.FlowEndAttachments(nested(t, asm.Scope, "f"))
	if len(flows) != 2 || flows[0].Index == nil || flows[1].Index != nil {
		t.Errorf("flow end attachments = %+v, want the index on the source alone", flows)
	}
}
