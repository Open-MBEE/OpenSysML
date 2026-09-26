package sysmlv1

import (
	"fmt"
	"testing"
)

// A document's view tree is DocGen's: every view-typed property of a view is
// a child section in declaration order, and only a composite (or shared)
// property's view is entered for its own children; a plain reference is a
// leaf, and the exposures on the referencing property are not the view's.
func TestDocGenViewTreeFollowsAggregation(t *testing.T) {
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_doc" name="Doc">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_owned" name="owned" type="_owned" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_linked" name="linked" type="_linked">
        <ownedComment xmi:type="uml:Comment" xmi:id="_c" body="a note"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_shared" name="shared" type="_shared" aggregation="shared"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_owned" name="Owned">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_leaf1" name="leaf" type="_leaf" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_linked" name="Linked">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_leaf2" name="leaf" type="_leaf" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_shared" name="Shared">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_leaf3" name="leaf" type="_leaf" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_leaf" name="Leaf"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_target" name="Target"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_dep_owned" client="_p_owned" supplier="_target"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_dep_linked" client="_p_linked" supplier="_target"/>
  </uml:Model>
  <Document_Profile_:Document xmi:id="_st_doc" base_Class="_doc"/>
  <Document_Profile_:view xmi:id="_st_owned" base_Class="_owned"/>
  <Document_Profile_:view xmi:id="_st_linked" base_Class="_linked"/>
  <Document_Profile_:view xmi:id="_st_shared" base_Class="_shared"/>
  <Document_Profile_:view xmi:id="_st_leaf" base_Class="_leaf"/>
  <sysml:Expose xmi:id="_st_expose_owned" base_Dependency="_dep_owned"/>
  <sysml:Expose xmi:id="_st_expose_linked" base_Dependency="_dep_linked"/>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Documents) != 1 {
		t.Fatalf("%d documents, want 1", len(m.Documents))
	}
	root := m.Documents[0].Root
	names := func(vs []*DocGenView) []string {
		var out []string
		for _, v := range vs {
			out = append(out, v.Class.Name)
		}
		return out
	}
	if got := names(root.Children); len(got) != 3 || got[0] != "Owned" || got[1] != "Linked" || got[2] != "Shared" {
		t.Fatalf("sections = %v, want [Owned Linked Shared]", got)
	}
	owned, linked, shared := root.Children[0], root.Children[1], root.Children[2]
	if got := names(owned.Children); len(got) != 1 || got[0] != "Leaf" {
		t.Errorf("composite view's children = %v, want [Leaf]", got)
	}
	if got := names(shared.Children); len(got) != 1 || got[0] != "Leaf" {
		t.Errorf("shared view's children = %v, want [Leaf]", got)
	}
	if len(linked.Children) != 0 {
		t.Errorf("referenced view's children = %v, want none", names(linked.Children))
	}
	if len(owned.Exposed) != 1 || owned.Exposed[0].Element == nil || owned.Exposed[0].Element.ID != "_target" {
		t.Errorf("composite property's exposure = %+v, want Target", owned.Exposed)
	}
	if len(linked.Exposed) != 0 {
		t.Errorf("referencing property's exposure = %+v, want none", linked.Exposed)
	}
	for _, v := range []*DocGenView{root, owned, linked, shared} {
		if len(v.Malformed) != 0 {
			t.Errorf("%s malformed: %v", v.Class.Name, v.Malformed)
		}
	}
}

// A control flow whose source or target names no node makes the whole chain
// unreadable: the walk refuses it instead of ending cleanly where the edge is lost.
func TestDocGenChainRefusesDanglingFlows(t *testing.T) {
	const method = `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Activity" xmi:id="_act" name="Method">
      <node xmi:type="uml:InitialNode" xmi:id="_init"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_collect" name="Collect"/>
      <node xmi:type="uml:StructuredActivityNode" xmi:id="_table" name="Table"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_collect"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e2" %s/>
    </packagedElement>
  </uml:Model>
  <Document_Profile_:CollectOwnedElements xmi:id="_st_c" base_Element="_collect"/>
  <Document_Profile_:TableStructure xmi:id="_st_t" base_Element="_table"/>
</xmi:XMI>`
	for _, tc := range []struct{ edge, want string }{
		{`source="_collect" target="_table"`, ""},
		{`source="_collect" target="_missing"`, `ControlFlow _e2's target "_missing" names no node`},
		{`source="_missing" target="_table"`, `ControlFlow _e2's source "_missing" names no node`},
		{`source="_collect"`, `ControlFlow _e2 has no target`},
	} {
		m, err := Parse([]byte(fmt.Sprintf(method, tc.edge)))
		if err != nil {
			t.Fatal(err)
		}
		steps, end := m.DocGenChain(m.Lookup("_act"))
		if end != tc.want {
			t.Errorf("edge %s: chain ended with %q, want %q", tc.edge, end, tc.want)
		}
		if want := 2; tc.want == "" && len(steps) != want {
			t.Errorf("edge %s: %d steps, want %d", tc.edge, len(steps), want)
		}
		if tc.want != "" && steps != nil {
			t.Errorf("edge %s: a refused chain still has steps %v", tc.edge, steps)
		}
	}
}

// The 2022x collaborator schema places a paragraph by sectionId (the view it
// sits in) and orders it by parentId (the preceding paragraph's comment), with
// viewId naming the document class; a parentId
// that resolves to nothing heads the order.
func TestDocGenParagraphsRead2022xTags(t *testing.T) {
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi"
         xmlns:Document_View_Collaborator_Profile="http://www.magicdraw.com/schemas/manual/Document_View_Collaborator_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_doc" name="Doc">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_top" name="top" type="_view_top" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_view_top" name="Top">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_sec" name="sec" type="_view_sec" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_view_sec" name="Sec">
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_head" body="heads the order"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_first" body="first"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_second" body="second"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_img" body="figure"/>
    </packagedElement>
  </uml:Model>
  <Document_Profile_:Document xmi:id="_st_doc" base_Class="_doc"/>
  <sysml:View xmi:id="_st_top" base_Class="_view_top"/>
  <sysml:View xmi:id="_st_sec" base_Class="_view_sec"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_p_head" base_Element="_c_head" documentId="mms-1" branchId="mms-2" viewId="_doc" sectionId="_view_sec" parentId="mms-gone"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_p1" base_Element="_c_first" documentId="mms-1" viewId="_doc" sectionId="_view_sec"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_p2" base_Element="_c_second" documentId="mms-1" viewId="_doc" sectionId="_view_sec" parentId="_c_first"/>
  <Document_View_Collaborator_Profile:CollaboratorImageParagraph xmi:id="_st_p3" base_Element="_c_img" documentId="mms-1" viewId="_doc" sectionId="_view_sec" parentId="_c_second"/>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Documents) != 1 {
		t.Fatalf("%d documents, want 1", len(m.Documents))
	}
	sec := m.Documents[0].Root.Children[0].Children[0]
	if sec.Class.ID != "_view_sec" {
		t.Fatalf("section = %s, want _view_sec", sec.Class.ID)
	}
	want := []string{"_c_head", "_c_first", "_c_second", "_c_img"}
	if len(sec.Paragraphs) != len(want) {
		t.Fatalf("%d paragraphs, want %d", len(sec.Paragraphs), len(want))
	}
	for i, p := range sec.Paragraphs {
		if p.Comment == nil || p.Comment.ID != want[i] {
			t.Fatalf("paragraph %d = %+v, want comment %s", i, p, want[i])
		}
		if p.Malformed != "" {
			t.Errorf("paragraph %s malformed: %s", want[i], p.Malformed)
		}
	}
	if !sec.Paragraphs[3].Image {
		t.Error("the image paragraph did not report Image")
	}
	for i, placed := range []bool{false, false, true, true} {
		if sec.Paragraphs[i].Placed != placed {
			t.Errorf("paragraph %s: Placed = %v, want %v", want[i], sec.Paragraphs[i].Placed, placed)
		}
	}
	if len(m.StrayParagraphs) != 0 {
		t.Errorf("%d stray paragraphs, want none", len(m.StrayParagraphs))
	}
}

// A section view two documents share does not leak a paragraph: the
// paragraph's viewId names its document, in the 2022x schema as in the old, so
// it is placed in that document's copy of the section and not the other's.
func TestDocGenParagraphsStayInTheirDocument(t *testing.T) {
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi"
         xmlns:Document_View_Collaborator_Profile="http://www.magicdraw.com/schemas/manual/Document_View_Collaborator_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_doc_a" name="Doc A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_atop" name="top" type="_view_atop" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_doc_b" name="Doc B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_btop" name="top" type="_view_btop" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_view_atop" name="A Top">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_secA" name="sec" type="_sec" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_view_btop" name="B Top">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_secB" name="sec" type="_sec" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_sec" name="Shared Sec">
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_a" body="a's note"/>
    </packagedElement>
  </uml:Model>
  <Document_Profile_:Document xmi:id="_st_da" base_Class="_doc_a"/>
  <Document_Profile_:Document xmi:id="_st_db" base_Class="_doc_b"/>
  <sysml:View xmi:id="_st_atop" base_Class="_view_atop"/>
  <sysml:View xmi:id="_st_btop" base_Class="_view_btop"/>
  <sysml:View xmi:id="_st_sec" base_Class="_sec"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_pa" base_Element="_c_a" documentId="mms-1" viewId="_doc_a" sectionId="_sec"/>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Documents) != 2 {
		t.Fatalf("%d documents, want 2", len(m.Documents))
	}
	var a, b *DocGenView
	for _, d := range m.Documents {
		sec := d.Root.Children[0].Children[0]
		switch d.Root.Class.ID {
		case "_doc_a":
			a = sec
		case "_doc_b":
			b = sec
		}
	}
	if len(a.Paragraphs) != 1 || a.Paragraphs[0].Comment.ID != "_c_a" {
		t.Errorf("doc A's section paragraphs = %+v, want _c_a", a.Paragraphs)
	}
	if len(b.Paragraphs) != 0 {
		t.Errorf("doc B's section paragraphs = %+v, want none", b.Paragraphs)
	}
	if len(m.StrayParagraphs) != 0 {
		t.Errorf("%d stray paragraphs, want none", len(m.StrayParagraphs))
	}
}

// A 2022x export may point a paragraph's viewId at the document's top view
// rather than the Document class; the paragraph still lands in its section.
func TestDocGenParagraphsTopViewId(t *testing.T) {
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi"
         xmlns:Document_View_Collaborator_Profile="http://www.magicdraw.com/schemas/manual/Document_View_Collaborator_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_doc" name="Doc">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_top" name="top" type="_top" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_top" name="Top">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_sec" name="sec" type="_sec" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_sec" name="Sec">
      <ownedComment xmi:type="uml:Comment" xmi:id="_c" body="note"/>
    </packagedElement>
  </uml:Model>
  <Document_Profile_:Document xmi:id="_st_doc" base_Class="_doc"/>
  <sysml:View xmi:id="_st_top" base_Class="_top"/>
  <sysml:View xmi:id="_st_sec" base_Class="_sec"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_p" base_Element="_c" documentId="mms-1" viewId="_top" sectionId="_sec"/>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Documents) != 1 {
		t.Fatalf("%d documents, want 1", len(m.Documents))
	}
	sec := m.Documents[0].Root.Children[0].Children[0]
	if len(sec.Paragraphs) != 1 || sec.Paragraphs[0].Comment.ID != "_c" {
		t.Errorf("section paragraphs = %+v, want _c", sec.Paragraphs)
	}
	if len(m.StrayParagraphs) != 0 {
		t.Errorf("%d stray paragraphs, want none", len(m.StrayParagraphs))
	}
}

// A view referred to first without aggregation and then as composite still has
// its children in the document's view tree, so their paragraphs are placed.
func TestDocGenViewTreeEntersACompositeAfterAReference(t *testing.T) {
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi"
         xmlns:Document_View_Collaborator_Profile="http://www.magicdraw.com/schemas/manual/Document_View_Collaborator_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_doc" name="Doc">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_preview" name="preview" type="_a"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_chapter" name="chapter" type="_a" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_b" name="b" type="_b" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedComment xmi:type="uml:Comment" xmi:id="_c" body="note"/>
    </packagedElement>
  </uml:Model>
  <Document_Profile_:Document xmi:id="_st_doc" base_Class="_doc"/>
  <sysml:View xmi:id="_st_a" base_Class="_a"/>
  <sysml:View xmi:id="_st_b" base_Class="_b"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_p" base_Element="_c" documentId="mms-1" viewId="_b" sectionId="_b"/>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Documents) != 1 || len(m.Documents[0].Root.Children) != 2 {
		t.Fatalf("documents = %+v, want one with two top views", m.Documents)
	}
	chapter := m.Documents[0].Root.Children[1]
	if len(chapter.Children) != 1 {
		t.Fatalf("the chapter has %d children, want B", len(chapter.Children))
	}
	if b := chapter.Children[0]; len(b.Paragraphs) != 1 || b.Paragraphs[0].Comment.ID != "_c" {
		t.Errorf("B's paragraphs = %+v, want _c", b.Paragraphs)
	}
	if len(m.StrayParagraphs) != 0 {
		t.Errorf("%d stray paragraphs, want none", len(m.StrayParagraphs))
	}
}

// A paragraph's predecessor tag is kept as written: one naming a paragraph of
// the view places it as a follower; one of the publisher's generated-item form
// is read into its anchor kind and target; any other heads the order unanchored.
func TestDocGenParagraphAnchors(t *testing.T) {
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi"
         xmlns:Document_View_Collaborator_Profile="http://www.magicdraw.com/schemas/manual/Document_View_Collaborator_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_doc" name="Doc">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_sec" name="sec" type="_view_sec" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_view_sec" name="Sec">
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_intro" body="intro"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_after" body="after the figure"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_next" body="next"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_other" body="after an unknown item"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_gone" body="after a gone paragraph"/>
    </packagedElement>
  </uml:Model>
  <Document_Profile_:Document xmi:id="_st_doc" base_Class="_doc"/>
  <sysml:View xmi:id="_st_sec" base_Class="_view_sec"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_intro" base_Element="_c_intro" documentId="mms-1" viewId="_doc" ownerId="_view_sec"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_after" base_Element="_c_after" documentId="mms-1" viewId="_doc" ownerId="_view_sec" siblingId="Containment_DiagramMainImage__d1"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_next" base_Element="_c_next" documentId="mms-1" viewId="_doc" ownerId="_view_sec" siblingId="_c_after"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_other" base_Element="_c_other" documentId="mms-1" viewId="_doc" ownerId="_view_sec" siblingId="Containment_TableMainImage__t1"/>
  <Document_View_Collaborator_Profile:CollaboratorParagraph xmi:id="_st_gone" base_Element="_c_gone" documentId="mms-1" viewId="_doc" ownerId="_view_sec" siblingId="_2022x_1_gone"/>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	sec := m.Documents[0].Root.Children[0]
	got := map[string]*DocGenParagraph{}
	for _, p := range sec.Paragraphs {
		got[p.Comment.ID] = p
	}
	if len(got) != 5 {
		t.Fatalf("%d paragraphs, want 5", len(got))
	}
	cases := []struct {
		id, predecessor string
		anchor          *DocGenAnchor
		placed          bool
	}{
		{"_c_intro", "", nil, false},
		{"_c_after", "Containment_DiagramMainImage__d1", &DocGenAnchor{Kind: DiagramMainImage, Target: "d1"}, false},
		{"_c_next", "_c_after", nil, true},
		{"_c_other", "Containment_TableMainImage__t1", &DocGenAnchor{Kind: "TableMainImage", Target: "t1"}, false},
		{"_c_gone", "_2022x_1_gone", nil, false},
	}
	for _, c := range cases {
		p := got[c.id]
		if p.Predecessor != c.predecessor {
			t.Errorf("%s: Predecessor = %q, want %q", c.id, p.Predecessor, c.predecessor)
		}
		if p.Placed != c.placed {
			t.Errorf("%s: Placed = %v, want %v", c.id, p.Placed, c.placed)
		}
		switch {
		case c.anchor == nil && p.Anchor != nil:
			t.Errorf("%s: Anchor = %+v, want none", c.id, *p.Anchor)
		case c.anchor != nil && (p.Anchor == nil || *p.Anchor != *c.anchor):
			t.Errorf("%s: Anchor = %+v, want %+v", c.id, p.Anchor, *c.anchor)
		}
	}
	for i, p := range sec.Paragraphs {
		if p.Comment.ID == "_c_next" && sec.Paragraphs[i-1].Comment.ID != "_c_after" {
			t.Errorf("_c_next follows %s, want _c_after", sec.Paragraphs[i-1].Comment.ID)
		}
	}
}
