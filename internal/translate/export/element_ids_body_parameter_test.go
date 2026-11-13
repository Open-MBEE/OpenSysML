package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A body's parameter (`in i` of `{ in i : Instrument; i.mass }`) declares no
// node of its own, so its symbol is keyed by the body; the ids answer it the
// parameter the conversion writes, named `i`, not the reference owning the body.
func TestElementIDsOfBodyParameterIsTheWrittenParameter(t *testing.T) {
	sf := source.New("body_parameter.sysml", []byte(`package P {
    part def Instrument { attribute mass : ScalarValues::Real; }
    part def R {
        ref part instruments : Instrument[0..*];
        attribute masses : ScalarValues::Real[0..*] = instruments->collect { in i : Instrument; i.mass };
    }
}
`))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: sf, Root: root}}, IDQualifiedName)
	if err != nil || len(encoders) == 0 {
		t.Fatalf("modelToRDF: %v", err)
	}
	ids, err := identityIDsOf(res, encoders, []bool{false})
	if err != nil {
		t.Fatalf("identityIDsOf: %v", err)
	}
	matches := res.Index().LookupQualified("P::R::masses")
	if len(matches) != 1 {
		t.Fatalf("P::R::masses names %d declarations, want 1", len(matches))
	}
	var bodyOf func(n ast.Node) *ast.BodyExpr
	bodyOf = func(n ast.Node) *ast.BodyExpr {
		if b, ok := n.(*ast.BodyExpr); ok {
			return b
		}
		parts, _ := ast.ExpressionOperands(n)
		for _, part := range parts {
			if b := bodyOf(part.Node); b != nil {
				return b
			}
		}
		return nil
	}
	body := bodyOf(matches[0].Decl.(*ast.Usage).Value)
	if body == nil {
		t.Fatal("the value of masses writes no expression body")
	}
	element, membership, ok := ids.OfBodyParameter(sf.Name(), body, "i")
	if !ok {
		t.Fatal("OfBodyParameter answers nothing for i")
	}
	var written string
	for _, subject := range graph.Subjects() {
		for _, name := range graph.Objects(subject, rdf.SysML+"declaredName") {
			if name.Value != "i" {
				continue
			}
			for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
				written = id.Value
			}
		}
	}
	if written == "" {
		t.Fatal("the conversion writes no element named i")
	}
	if element != written {
		t.Errorf("OfBodyParameter = %q, the conversion wrote %q for i", element, written)
	}
	if !strings.HasSuffix(membership, rdf.OwningMembershipSuffix) {
		t.Errorf("membership = %q, want the parameter's owning membership", membership)
	}
	if reference, ok := ids.OfDeclaration(sf.Name(), body, ""); !ok || reference == element {
		t.Errorf("the body's reference %q (ok %v) and its parameter %q must be distinct elements", reference, ok, element)
	}
	if _, _, ok := ids.OfBodyParameter(sf.Name(), body, "j"); ok {
		t.Error("OfBodyParameter answers an id for a parameter the body does not declare")
	}
}
