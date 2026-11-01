package export

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// calleeText takes the collapsed function and the owned chain feature as one
// statement: the function is the chain's last feature, or the chain's whole
// spelling when unresolved, and a function that merely shares its last
// segment's spelling is a second callee, refused.
func TestCalleeTextChainAgreesByIdentity(t *testing.T) {
	node := rdf.IRI("urn:x:call")
	membership := rdf.IRI("urn:x:call_pfunction_om")
	chain := rdf.IRI("urn:x:call_pfunction")
	foo, other := rdf.IRI("urn:x:Service__foo"), rdf.IRI("urn:x:Other__foo")
	base, typeF := rdf.IRI("urn:x:Service__base"), rdf.IRI("urn:x:Service__F")
	calculation, evaluation := rdf.IRI("urn:x:Calculations__Calculation"), rdf.IRI("urn:x:Performances__Evaluation")
	var typed, inherited bool
	build := func(function rdf.Term, segments ...rdf.Term) *decoder {
		g := rdf.NewGraph()
		if typed {
			g.Add(foo, rdf.SysMLTerm(relationshipProperty[ast.RelTyping]), typeF)
		}
		if inherited {
			g.Add(foo, rdf.SysMLTerm(relationshipProperty[ast.RelSubsets]), base)
			g.Add(base, rdf.SysMLTerm(relationshipProperty[ast.RelTyping]), typeF)
		}
		g.Add(node, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(mInvocation))
		g.Add(node, rdf.SysMLTerm(pFunction), function)
		g.Add(node, rdf.SysMLTerm(pOwnedRelationship), membership)
		g.Add(membership, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(mOwningMembership))
		g.Add(membership, rdf.SysMLTerm(pMemberElement), chain)
		g.Add(chain, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(mFeature))
		for _, s := range segments {
			g.Add(chain, rdf.SysMLTerm(pChainingFeature), s)
		}
		d := newDecoder(g, map[rdf.Term]string{
			node: rdf.SysML + mInvocation, membership: rdf.SysML + mOwningMembership, chain: rdf.SysML + mFeature,
			foo: rdf.SysML + "CalculationUsage", base: rdf.SysML + "CalculationUsage", typeF: rdf.SysML + "CalculationDefinition",
		}, nil, nil, nil)
		for iri, qname := range map[rdf.Term]string{foo: "Service::foo", other: "Other::foo", base: "Service::base", typeF: "Service::F"} {
			g.Add(iri, rdf.SysMLTerm(pQualifiedName), rdf.String(qname))
			g.Add(iri, rdf.SysMLTerm(pDeclaredName), rdf.String(qname[strings.LastIndex(qname, ":")+1:]))
			d.byIRI[iri.Value] = &element{iri: iri.Value, qname: qname}
		}
		for iri, qname := range map[rdf.Term]string{calculation: "Calculations::Calculation", evaluation: "Performances::Evaluation"} {
			g.Add(iri, rdf.SysMLTerm(pQualifiedName), rdf.String(qname))
			g.Add(iri, rdf.SysMLTerm(pDeclaredName), rdf.String(qname[strings.LastIndex(qname, ":")+1:]))
			d.byIRI[iri.Value] = &element{iri: iri.Value, qname: qname, library: true}
		}
		return d
	}
	in := &element{}
	cases := []struct {
		name      string
		function  rdf.Term
		segments  []rdf.Term
		refused   bool
		typed     bool
		inherited bool
	}{
		{"same feature", foo, []rdf.Term{rdf.String("s"), foo}, false, false, false},
		{"other feature of the same name", other, []rdf.Term{rdf.String("s"), foo}, true, false, false},
		{"the type of the last feature", typeF, []rdf.Term{rdf.String("s"), foo}, false, true, false},
		{"the type the last feature inherits", typeF, []rdf.Term{rdf.String("s"), foo}, false, false, true},
		{"the typing the untyped calc implies", calculation, []rdf.Term{rdf.String("s"), foo}, false, false, false},
		{"a typed feature's implied typing still holds", calculation, []rdf.Term{rdf.String("s"), foo}, false, true, false},
		{"another library Function", evaluation, []rdf.Term{rdf.String("s"), foo}, true, false, false},
		{"whole spelling", rdf.String("s.foo"), []rdf.Term{rdf.String("s"), rdf.String("foo")}, false, false, false},
		{"longer spelling ending alike", rdf.String("myfoo"), []rdf.Term{rdf.String("s"), rdf.String("foo")}, true, false, false},
		{"qualified name ending alike", rdf.String("Other::foo"), []rdf.Term{rdf.String("s"), rdf.String("foo")}, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			typed, inherited = c.typed, c.inherited
			_, _, err := build(c.function, c.segments...).calleeText(node, in)
			var unsupported *UnsupportedError
			if refused := errors.As(err, &unsupported); refused != c.refused {
				t.Fatalf("refused = %v, want %v (err %v)", refused, c.refused, err)
			}
		})
	}
}
