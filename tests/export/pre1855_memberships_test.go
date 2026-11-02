package export_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// Turtle written before the writer stated a Membership's source, target,
// relatedElement and memberName reads back from its source text untouched: a
// statement the graph omits and the candidate derives is no evidence the text
// is stale. A memberName the graph does state differently still is.
func TestPre1855MembershipsKeepVerbatimText(t *testing.T) {
	turtle, err := os.ReadFile(filepath.Join("testdata", "pre1855_memberships.ttl"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "pre1855_memberships.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	graph := string(turtle)
	// The alias is the one membership whose name the text itself states; every
	// other membership predates the writer stating its ends and names.
	memberships := 0
	for _, block := range strings.Split(graph, "\n\n") {
		lines := strings.SplitN(block, "\n", 3)
		if len(lines) < 3 || !strings.HasSuffix(lines[1], "Membership ;") || lines[0] == "elmt:Vehicle__Frame" {
			continue
		}
		memberships++
		for _, stated := range []string{"sysml:source", "sysml:target", "sysml:relatedElement", "json:relatedElement", "sysml:memberName", "sysml:memberShortName"} {
			if strings.Contains(block, stated) {
				t.Fatalf("the fixture states %s; it no longer predates the membership statements:\n%s", stated, block)
			}
		}
	}
	if memberships < 10 {
		t.Fatalf("the fixture has %d memberships, too few to carry the test", memberships)
	}
	membership := "elmt:Vehicle__car_om\n    a sysml:OwningMembership ;\n"
	if !strings.Contains(graph, membership) {
		t.Fatal("the fixture's membership of car is not where the control looks")
	}
	for _, comment := range []string{"// Imports come first.", "/* The chassis carries everything else. */", "// Two tonnes."} {
		if !bytes.Contains(want, []byte(comment)) {
			t.Fatalf("the fixture's notation lacks %q", comment)
		}
	}

	back, err := convert.Convert("pre1855.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("pre-membership-statement Turtle refused: %v", err)
	}
	if !bytes.Equal(back, want) {
		t.Errorf("the verbatim text did not survive:\n--- want\n%s\n--- got\n%s", want, back)
	}

	// Control: a memberName the graph states against the text is a
	// disagreement over the membership, which demotes the text of the namespace
	// that owns it — the package head and the file comment before it — and
	// leaves the members' text alone, as it did before the statements existed.
	stated := strings.Replace(graph, membership, membership+"    sysml:memberName \"cart\" ;\n", 1)
	if stated == graph {
		t.Fatal("the control did not state a memberName")
	}
	back, err = convert.Convert("stated.ttl", []byte(stated), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("stated memberName refused: %v", err)
	}
	if bytes.Contains(back, []byte("// A small model")) {
		t.Errorf("a stated memberName that disagrees with the text did not demote it:\n%s", back)
	}
	for _, comment := range []string{"/* The chassis carries everything else. */", "// Two tonnes."} {
		if !bytes.Contains(back, []byte(comment)) {
			t.Errorf("a disagreement over one membership demoted a member's text (%q):\n%s", comment, back)
		}
	}
}
