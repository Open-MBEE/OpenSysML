package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

func relinkedProperty(t *testing.T, graph []byte, subject, property, from, to string) []byte {
	t.Helper()
	blocks := strings.Split(string(graph), "\n\n")
	needle := "sysml:" + property + " " + from
	replacement := "sysml:" + property + " " + to
	found := 0
	for i, block := range blocks {
		if !strings.Contains(block, subject+"\n") {
			continue
		}
		if strings.Count(block, needle) != 1 {
			continue
		}
		blocks[i] = strings.Replace(block, needle, replacement, 1)
		found++
	}
	if found != 1 {
		t.Fatalf("subject %q has property %q %d times, want once\n%s", subject, property, found, graph)
	}
	return []byte(strings.Join(blocks, "\n\n"))
}

// relinkedMember relinks the member a Membership names — memberElement, target and relatedElement,
// the json annotation of the latter included — from one element to another.
func relinkedMember(t *testing.T, graph []byte, membership, owner, from, to string) []byte {
	t.Helper()
	graph = relinkedProperty(t, graph, membership, "memberElement", from, to)
	graph = relinkedProperty(t, graph, membership, "target", from, to)
	graph = relinkedProperty(t, graph, membership, "relatedElement", owner+", "+from, owner+", "+to)
	id := func(term string) string { return term[strings.Index(term, ":")+1:] }
	return relinkedWithin(t, graph, membership,
		`json:relatedElement "[{\"@id\":\"`+id(owner)+`\"},{\"@id\":\"`+id(from)+`\"}]"`,
		`json:relatedElement "[{\"@id\":\"`+id(owner)+`\"},{\"@id\":\"`+id(to)+`\"}]"`)
}

// relinkedWithin replaces from with to inside the block of one subject only.
func relinkedWithin(t *testing.T, graph []byte, subject, from, to string) []byte {
	t.Helper()
	blocks := strings.Split(string(graph), "\n\n")
	found := 0
	for i, block := range blocks {
		if strings.Contains(block, subject+"\n") && strings.Contains(block, from) {
			blocks[i] = strings.ReplaceAll(block, from, to)
			found++
		}
	}
	if found != 1 {
		t.Fatalf("subject %q states %q in %d blocks, want one\n%s", subject, from, found, graph)
	}
	return []byte(strings.Join(blocks, "\n\n"))
}

// TestMembershipEndsFollowTheMember: an alias to an unlinked name states no ends, `done;` and
// `first x;` state theirs, and a graph whose ends disagree with its ownership is refused.
func TestMembershipEndsFollowTheMember(t *testing.T) {
	src := `package P {
	part def A;
	part def B;
	alias Alias for A;
	alias Lost for Missing;
	alias Self for P;
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
		case strings.Contains(block, `sysml:memberName "Self"`):
			// Owner and member are one element, so the set of related elements has one.
			if !strings.Contains(block, "sysml:relatedElement elmt:P ;") || !strings.Contains(block, "sysml:target elmt:P ;") {
				t.Errorf("a namespace aliasing itself has one related element\n%s", block)
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
	for _, block := range strings.Split(string(structural), "\n\n") {
		if !strings.Contains(block, `sysml:memberName "Alias"`) {
			continue
		}
		alias := strings.Fields(block)[0]
		moved := relinkedProperty(t, structural, alias, "target", "elmt:P__A", "elmt:P__B")
		refusedAsUnsupported(t, "alias-target", moved, "its sysml:target is <urn:sysmlv2:element:P__B>, but a membership's ends are its owning namespace <urn:sysmlv2:element:P> and its member <urn:sysmlv2:element:P__A>")
	}
	if back := backFromTheGraphAlone(t, string(structural)); !strings.Contains(back, "alias Alias for A;") || !strings.Contains(back, "alias Self for P;") {
		t.Errorf("the graph with its ends comes back\n%s", back)
	}
}
