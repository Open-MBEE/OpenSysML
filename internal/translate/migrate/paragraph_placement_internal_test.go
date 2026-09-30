package migrate

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

func TestSectionInsertKeepsFigureMarks(t *testing.T) {
	d1, d2 := &sysmlv1.Diagram{ID: "d1"}, &sysmlv1.Diagram{ID: "d2"}
	sec := &sectionPlan{}
	sec.content = append(sec.content, &contentPlan{kind: "Paragraph", text: "head"})
	sec.content = append(sec.content, &contentPlan{kind: "Diagram"})
	sec.mark(d1)
	sec.content = append(sec.content, &contentPlan{kind: "Paragraph", text: "caption"})
	sec.content = append(sec.content, &contentPlan{kind: "Table"})
	sec.mark(d2)

	sec.insert(sec.figures[0].at, &contentPlan{kind: "Paragraph", text: "after d1"}, &contentPlan{kind: "Paragraph", text: "follower"})
	sec.insert(sec.figures[1].at, &contentPlan{kind: "Paragraph", text: "after d2"})

	var got []string
	for _, cp := range sec.content {
		if cp.kind == "Paragraph" {
			got = append(got, cp.text)
		} else {
			got = append(got, cp.kind)
		}
	}
	want := []string{"head", "Diagram", "after d1", "follower", "caption", "Table", "after d2"}
	if len(got) != len(want) {
		t.Fatalf("content = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("content = %q, want %q", got, want)
		}
	}
	if sec.figures[0].at != 4 || sec.figures[1].at != 7 {
		t.Errorf("marks after insertion = %d, %d; want 4, 7", sec.figures[0].at, sec.figures[1].at)
	}
}

func TestParagraphGroupsRunFromEachHead(t *testing.T) {
	head1 := &sysmlv1.DocGenParagraph{}
	follower1 := &sysmlv1.DocGenParagraph{Placed: true, Predecessor: "head1"}
	follower2 := &sysmlv1.DocGenParagraph{Placed: true, Predecessor: "follower1"}
	head2 := &sysmlv1.DocGenParagraph{Predecessor: "Containment_DiagramMainImage__d1", Anchor: &sysmlv1.DocGenAnchor{Kind: sysmlv1.DiagramMainImage, Target: "d1"}}
	follower3 := &sysmlv1.DocGenParagraph{Placed: true, Predecessor: "head2"}
	groups := paragraphGroups([]*sysmlv1.DocGenParagraph{head1, follower1, follower2, head2, follower3})
	if len(groups) != 2 || len(groups[0]) != 3 || len(groups[1]) != 2 {
		t.Fatalf("groups = %v, want [3 2]", groups)
	}
	if groups[0][0] != head1 || groups[0][2] != follower2 || groups[1][0] != head2 || groups[1][1] != follower3 {
		t.Errorf("groups do not run from each head in order: %v", groups)
	}
}
