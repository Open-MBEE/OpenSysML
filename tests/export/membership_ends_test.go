package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// TestMembershipEndsFollowTheMember covers the ends a Membership states: an alias to a name
// the graph cannot link has no element to be its target, so none is stated; `done;` and
// `first x;` memberships state theirs; a graph whose ends disagree with its ownership is refused.
func TestMembershipEndsFollowTheMember(t *testing.T) {
	src := `package P {
	part def A;
	part def B;
	alias Alias for A;
	alias Lost for Missing;
	action act {
		action x;
		first x;
		done;
	}
}`
	graph, err := convert.Convert("ends.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, block := range strings.Split(string(graph), "\n\n") {
		switch {
		case strings.Contains(block, `sysml:memberName "Lost"`):
			if strings.Contains(block, "sysml:target") || strings.Contains(block, "sysml:relatedElement") {
				t.Errorf("an alias to an unlinked name states no ends\n%s", block)
			}
		case strings.Contains(block, `sysml:memberName "Alias"`):
			if !strings.Contains(block, "sysml:target elmt:P__A") || !strings.Contains(block, "sysml:relatedElement elmt:P, elmt:P__A") {
				t.Errorf("an alias states its member as its target\n%s", block)
			}
		case strings.Contains(block, `sysx:declaredKeyword "done"`):
			if !strings.Contains(block, "sysml:source elmt:P__act") || !strings.Contains(block, "sysml:target ") {
				t.Errorf("`done;` states its ends\n%s", block)
			}
		case strings.Contains(block, `sysx:declaredKeyword "first"`) && strings.Contains(block, "sysml:memberElement elmt:P__act__x"):
			if !strings.Contains(block, "sysml:source elmt:P__act") || !strings.Contains(block, "sysml:target elmt:P__act__x") {
				t.Errorf("`first x;` states its ends\n%s", block)
			}
		}
	}
	structural := withoutTriples(t, graph, "sysx:sourceText")
	relinked := relinkedProperty(t, structural, "elmt:P__A_om", "target", "elmt:P__A", "elmt:P__B")
	refusedAsUnsupported(t, "ends-target", relinked, "its sysml:target is <urn:sysmlv2:element:P__B>, but a membership's ends are its owning namespace <urn:sysmlv2:element:P> and its member <urn:sysmlv2:element:P__A>")
	extra := relinkedProperty(t, structural, "elmt:P__A_om", "relatedElement", "elmt:P, elmt:P__A", "elmt:P, elmt:P__A, elmt:P__B")
	extra = []byte(strings.Replace(string(extra), `json:relatedElement "[{\"@id\":\"P\"},{\"@id\":\"P__A\"}]"`,
		`json:relatedElement "[{\"@id\":\"P\"},{\"@id\":\"P__A\"},{\"@id\":\"P__B\"}]"`, 1))
	refusedAsUnsupported(t, "ends-related", extra, "its sysml:relatedElement is <urn:sysmlv2:element:P>, <urn:sysmlv2:element:P__A>, <urn:sysmlv2:element:P__B>")
	if back := backFromTheGraphAlone(t, string(structural)); !strings.Contains(back, "alias Alias for A;") {
		t.Errorf("the graph with its ends comes back\n%s", back)
	}
}
