package export

import (
	"errors"
	"testing"

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
	build := func(function rdf.Term, segments ...rdf.Term) *decoder {
		g := rdf.NewGraph()
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
		}, nil)
		for iri, qname := range map[rdf.Term]string{foo: "Service::foo", other: "Other::foo"} {
			g.Add(iri, rdf.SysMLTerm(pQualifiedName), rdf.String(qname))
			d.byIRI[iri.Value] = &element{iri: iri.Value, qname: qname}
		}
		return d
	}
	in := &element{}
	cases := []struct {
		name     string
		function rdf.Term
		segments []rdf.Term
		refused  bool
	}{
		{"same feature", foo, []rdf.Term{rdf.String("s"), foo}, false},
		{"other feature of the same name", other, []rdf.Term{rdf.String("s"), foo}, true},
		{"whole spelling", rdf.String("s.foo"), []rdf.Term{rdf.String("s"), rdf.String("foo")}, false},
		{"longer spelling ending alike", rdf.String("myfoo"), []rdf.Term{rdf.String("s"), rdf.String("foo")}, true},
		{"qualified name ending alike", rdf.String("Other::foo"), []rdf.Term{rdf.String("s"), rdf.String("foo")}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := build(c.function, c.segments...).calleeText(node, in)
			var unsupported *UnsupportedError
			if refused := errors.As(err, &unsupported); refused != c.refused {
				t.Fatalf("refused = %v, want %v (err %v)", refused, c.refused, err)
			}
		})
	}
}
