package runtime

import "testing"

// A root-level element's metaobject reads no owning namespace, membership or relationship: the
// document's RootNamespace is an element no model states. A nested element reads its own.
func TestMetaobjectRootLevelElementReadsNoOwnership(t *testing.T) {
	const src = `
package W { part def A; }
part def RootDef;
part rootPart : RootDef;
`
	for expr, want := range map[string]string{
		"W.metadata.owner":                     "()",
		"W.metadata.owningNamespace":           "()",
		"W.metadata.owningMembership":          "()",
		"W.metadata.owningRelationship":        "()",
		"RootDef.metadata.owningRelationship":  "()",
		"RootDef.metadata.owningNamespace":     "()",
		"rootPart.metadata.owningRelationship": "()",
		"rootPart.metadata.owningMembership":   "()",
		"W::A.metadata.owningNamespace":        "(meta(W : KerML::Kernel::Package))",
		"W::A.metadata.owningRelationship":     "(meta(W::<unnamed> : KerML::Root::OwningMembership))",
	} {
		_, got, err := evalDeclaredExpr(t, src, expr)
		if err != nil {
			t.Fatalf("%s failed: %v", expr, err)
		}
		if text := FormatTraceValue(got); text != want {
			t.Errorf("%s = %s, want %s", expr, text, want)
		}
	}
}
