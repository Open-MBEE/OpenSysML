package export_test

import (
	"strings"
	"testing"
)

// TestImpliedMembershipsStateTheirMemberNames: the memberships the conversion implies — a connector
// end's, a conjugated port definition's, a satisfy subject's — state memberName as the model derives it,
// an expression's referent Membership none, and an import states its source and target.
func TestImpliedMembershipsStateTheirMemberNames(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	import Q::X;
	package Q {
		part def X;
	}
	port def Pt;
	part def S {
		attribute a : Real;
	}
	part s : S;
	part u : S;
	connection c connect s to u;
	requirement def R;
	requirement r : R;
	part obs {
		satisfy r by s;
	}
	attribute v = 1 + s.a;
}
`
	turtle := roundTripsExactly(t, src)
	want := map[string][]string{
		"expr:P__c_pend0_om":           {`sysml:memberName "source"`},
		"expr:P__c_pend1_om":           {`sysml:memberName "target"`},
		"elmt:P__Pt_conjugated_om":     {`sysml:memberName "~Pt"`},
		"elmt:P__obs___400_subject_om": {`sysml:memberName "subj"`},
	}
	blocks := strings.Split(string(turtle), "\n\n")
	for subject, wants := range want {
		found := false
		for _, block := range blocks {
			if !strings.HasPrefix(block, subject+"\n") && !strings.HasPrefix(block, subject+" ") {
				continue
			}
			found = true
			for _, w := range wants {
				if !strings.Contains(block, w) {
					t.Errorf("%s lacks %s\n%s", subject, w, block)
				}
			}
		}
		if !found {
			t.Errorf("no block for %s", subject)
		}
	}
	imports := 0
	for _, block := range blocks {
		switch {
		case strings.Contains(block, "a sysml:NamespaceImport ;"):
			imports++
			if !strings.Contains(block, "sysml:source elmt:P ;") || !strings.Contains(block, "sysml:target <urn:sysmlv2:element:") {
				t.Errorf("a namespace import states its owner and the namespace as its ends\n%s", block)
			}
		case strings.Contains(block, "a sysml:MembershipImport ;"):
			imports++
			if !strings.Contains(block, "sysml:source elmt:P ;") || !strings.Contains(block, "sysml:target elmt:P__Q__X_om") {
				t.Errorf("a membership import states its owner and the imported membership as its ends\n%s", block)
			}
		}
	}
	if imports != 2 {
		t.Errorf("found %d import blocks, want 2", imports)
	}
	for _, block := range blocks {
		if strings.HasPrefix(block, "expr:P__obs___400_subject_pvalue_preferent") && strings.Contains(block, "sysml:memberName") {
			t.Errorf("a plain Membership an expression owns specifies no memberName\n%s", block)
		}
	}
}

// TestImportEndsMustAgreeWithWhatTheImportStates: an import whose stated source or target disagrees
// with its importOwningNamespace or the namespace or membership it imports is refused.
func TestImportEndsMustAgreeWithWhatTheImportStates(t *testing.T) {
	graph := roundTripsExactly(t, `package P {
	import Q::X;
	package Q {
		part def X;
	}
}
`)
	structural := withoutTriples(t, graph, "sysx:sourceText")
	relinked := relinkedProperty(t, structural, "elmt:P___400", "target", "elmt:P__Q__X_om", "elmt:P__Q")
	refusedAsUnsupported(t, "import-target", relinked,
		"its sysml:target is <urn:sysmlv2:element:P__Q>, but its imported namespace or membership is <urn:sysmlv2:element:P__Q__X_om>")
	moved := relinkedProperty(t, structural, "elmt:P___400", "source", "elmt:P", "elmt:P__Q")
	refusedAsUnsupported(t, "import-source", moved,
		"its sysml:source is <urn:sysmlv2:element:P__Q>, but its importOwningNamespace is <urn:sysmlv2:element:P>")
}
