package sysmlv1

import (
	"strings"
	"testing"
)

// diagramDocument wraps diagrams, serialized the way a tool nests them inside
// an xmi:Extension of the model, in a document with a block and a part.
func diagramDocument(diagrams string) []byte {
	return []byte(`<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:diagram="http://www.example.com/tool/diagram">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Package" xmi:id="_p" name="P">
      <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_a_b" name="b" type="_b"/>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_a_m" name="m">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedAttribute>
      </packagedElement>
      <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    </packagedElement>
    <xmi:Extension extender="Example UML Tool 1.0">
      <modelExtension>` + diagrams + `</modelExtension>
      <other xmi:type="tool:Setting" xmi:id="_setting" name="grid"/>
    </xmi:Extension>
  </uml:Model>
</xmi:XMI>`)
}

const bddDiagram = `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d_bdd" name="P BDD" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject xmi:id="_d_bdd_rep" type="SysML Block Definition Diagram" umlType="Class Diagram">
                <diagramContents xmi:id="_d_bdd_contents">
                  <usedObjects href="#_a"/>
                  <usedElements>_a</usedElements>
                  <usedElements> _a_b </usedElements>
                  <usedElements>_a</usedElements>
                  <usedElements>_missing</usedElements>
                  <usedObjects href="#_b"/>
                  <usedObjects href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
                  <usedObjects href="http://www.example.com/Unread.xmi#_elsewhere"/>
                  <binaryObject xsi:type="binary:StreamIdentityBinaryObject" streamContentID="BINARY-1"/>
                </diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>`

func parseDiagrams(t *testing.T, diagrams string) *Model {
	t.Helper()
	m, err := Parse(diagramDocument(diagrams))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func ids(refs []ElementRef) string {
	var out []string
	for _, r := range refs {
		s := r.ID
		if r.Element == nil {
			s += "?"
		}
		out = append(out, s)
	}
	return strings.Join(out, " ")
}

func TestDiagramIsRead(t *testing.T) {
	m := parseDiagrams(t, bddDiagram)
	if len(m.Diagrams) != 1 {
		t.Fatalf("diagrams = %+v", m.Diagrams)
	}
	d := m.Diagrams[0]
	if d.ID != "_d_bdd" || d.Name != "P BDD" || d.Kind != "SysML Block Definition Diagram" || d.UMLKind != "Class Diagram" {
		t.Errorf("diagram = %+v", d)
	}
	if d.OwnerID != "_p" || d.Owner != m.Lookup("_p") || d.Holder != m.Lookup("_m") || d.Extender != "Example UML Tool 1.0" {
		t.Errorf("owner = %q %v holder = %v extender = %q", d.OwnerID, d.Owner, d.Holder, d.Extender)
	}
	if !d.Represented() {
		t.Error("diagram is not represented")
	}
	// Each id once, in the order the tool listed them, resolved when defined;
	// hrefs and text spell the same element. An href into another document
	// resolves to the proxy the model already holds for it, or dangles.
	const realURI = "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"
	if got := ids(d.Shown); got != "_a _a_b _missing? _b "+realURI+" http://www.example.com/Unread.xmi#_elsewhere?" {
		t.Errorf("shown = %q", got)
	}
	if d.Shown[0].Element != m.Lookup("_a") || d.Shown[1].Element != m.Lookup("_a_b") || d.Shown[4].Element != m.Lookup(realURI) || !d.Shown[4].Element.IsProxy() {
		t.Errorf("shown elements = %+v", d.Shown)
	}
	// The diagram is no element of the model, and no skipped extension
	// content either; the rest of the extension still is.
	if m.Lookup("_d_bdd") != nil || m.Lookup("_d_bdd_rep") != nil {
		t.Error("diagram content was read as model elements")
	}
	var skipped []string
	for _, ext := range m.Extensions {
		for _, el := range ext.Elements {
			skipped = append(skipped, el.Type+" "+el.Name)
		}
	}
	if got := strings.Join(skipped, ", "); got != "tool:Setting grid" {
		t.Errorf("skipped extension content = %q", got)
	}
}

func TestDiagramWithoutOwner(t *testing.T) {
	m := parseDiagrams(t, `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d1" name="No Owner">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="SysML Activity Diagram" umlType="Activity Diagram">
                <diagramContents><usedElements>_a</usedElements></diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d2" name="Unknown Owner" ownerOfDiagram="_nowhere">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="Generic Table" umlType="Class Diagram">
                <diagramContents/>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>`)
	if len(m.Diagrams) != 2 {
		t.Fatalf("diagrams = %+v", m.Diagrams)
	}
	d1, d2 := m.Diagrams[0], m.Diagrams[1]
	if d1.OwnerID != "" || d1.Owner != nil || d1.Holder != m.Lookup("_m") || ids(d1.Shown) != "_a" {
		t.Errorf("d1 = %+v", d1)
	}
	if d2.OwnerID != "_nowhere" || d2.Owner != nil || len(d2.Shown) != 0 || d2.Kind != "Generic Table" || !d2.Represented() {
		t.Errorf("d2 = %+v", d2)
	}
}

func TestDiagramWithoutRepresentation(t *testing.T) {
	m := parseDiagrams(t, `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Bare" ownerOfDiagram="_a"/>`)
	if len(m.Diagrams) != 1 {
		t.Fatalf("diagrams = %+v", m.Diagrams)
	}
	d := m.Diagrams[0]
	if d.Represented() || d.Kind != "" || d.UMLKind != "" || len(d.Shown) != 0 || d.Owner != m.Lookup("_a") {
		t.Errorf("diagram = %+v", d)
	}
}

func TestDiagramDocumentationIsItsOwnComment(t *testing.T) {
	m := parseDiagrams(t, `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Documented" ownerOfDiagram="_a">
          <ownedComment xmi:type="uml:Comment" xmi:id="_c_about" body="About A." annotatedElement="_a"/>
          <ownedComment xmi:type="uml:Comment" xmi:id="_c_empty" body="  "/>
          <ownedComment xmi:type="uml:Comment" xmi:id="_c_own" annotatedElement="_d">
            <body> Shows A and B. </body>
          </ownedComment>
          <ownedComment xmi:type="uml:Comment" xmi:id="_c_second" body="Second."/>
        </ownedDiagram>
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d_bare" name="Bare" ownerOfDiagram="_a"/>`)
	if len(m.Diagrams) != 2 {
		t.Fatalf("diagrams = %+v", m.Diagrams)
	}
	if got := m.Diagrams[0].Documentation; got != "Shows A and B." {
		t.Errorf("documentation = %q, want the first own comment with text", got)
	}
	if got := m.Diagrams[1].Documentation; got != "" {
		t.Errorf("documentation of an uncommented diagram = %q", got)
	}
	for _, id := range []string{"_c_about", "_c_own"} {
		if m.Lookup(id) != nil {
			t.Errorf("the diagram's comment %s was read as a model element", id)
		}
	}
}

func TestDiagramKindFromTypeAlone(t *testing.T) {
	// A representation object naming only its type, and a binary object
	// ahead of it, still yield the diagram's kind.
	m := parseDiagrams(t, `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Typed" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="Content Diagram">
                <binaryObject xsi:type="binary:Stream"/>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>`)
	if d := m.Diagrams[0]; d.Kind != "Content Diagram" || d.UMLKind != "" {
		t.Errorf("diagram = %+v", d)
	}
}

func TestDiagramRepresentationIsNotATypedChild(t *testing.T) {
	// A comment the diagram owns ahead of its representation object carries
	// an xmi:type, which names no diagram kind; the contents the tool wrote
	// outside the representation object show nothing.
	m := parseDiagrams(t, `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Typed" ownerOfDiagram="_p">
          <ownedComment xmi:type="uml:Comment" xmi:id="_d_note" body="layout note"/>
          <xmi:Extension extender="Example UML Tool 1.0">
            <history><usedElements>_b</usedElements></history>
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="Generic Table">
                <diagramContents><usedElements>_a</usedElements></diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>`)
	if d := m.Diagrams[0]; d.Kind != "Generic Table" || d.UMLKind != "" || ids(d.Shown) != "_a" {
		t.Errorf("diagram = %+v", d)
	}
}

func TestDiagramRepresentationIsNotAPlainlyTypedChild(t *testing.T) {
	// A child carrying a plain type attribute ahead of the representation
	// object — a legend keyed to a classifier, a typed property — names a
	// UML type, not a diagram kind, and what it lists is not shown.
	m := parseDiagrams(t, `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Typed" ownerOfDiagram="_p">
          <ownedAttribute xmi:type="uml:Property" xmi:id="_d_p" name="frame" type="_b"/>
          <xmi:Extension extender="Example UML Tool 1.0">
            <legend type="_a"><usedElements>_b</usedElements></legend>
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="Generic Table">
                <diagramContents><usedElements>_a</usedElements></diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>`)
	if d := m.Diagrams[0]; d.Kind != "Generic Table" || d.UMLKind != "" || ids(d.Shown) != "_a" {
		t.Errorf("diagram = %+v", d)
	}
}

func TestDiagramRepresentationByUMLTypeAlone(t *testing.T) {
	// A tool that serializes the representation under its own tags is read
	// by the umlType it states; a plainly typed child ahead of it is not.
	m := parseDiagrams(t, `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Foreign" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <canvas type="_a"><usedElements>_b</usedElements></canvas>
            <notation:View type="Custom Table" umlType="Class Diagram">
              <children><usedElements>_a</usedElements></children>
            </notation:View>
          </xmi:Extension>
        </ownedDiagram>`)
	d := m.Diagrams[0]
	if d.Kind != "Custom Table" || d.UMLKind != "Class Diagram" || ids(d.Shown) != "_a" {
		t.Errorf("diagram = %+v", d)
	}
}

func TestDiagramWithoutRepresentationShowsNothing(t *testing.T) {
	// Without a representation object or a UML diagram type, a plainly typed
	// child gives the diagram no kind, and what such tool content lists is not
	// shown: only the representation's contents are.
	m := parseDiagrams(t, `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Bare" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <legend type="_a"><usedElements>_b</usedElements></legend>
            <history><usedObjects href="#_a"/></history>
          </xmi:Extension>
        </ownedDiagram>`)
	if d := m.Diagrams[0]; d.Represented() || d.Kind != "" || d.UMLKind != "" || len(d.Shown) != 0 {
		t.Errorf("diagram = %+v", d)
	}
}

func TestNestedDiagramKeepsItsOwnRepresentation(t *testing.T) {
	// A diagram serialized inside another is read as a diagram of its own;
	// its kind and contents are not the outer diagram's.
	m := parseDiagrams(t, `
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_outer" name="Outer" ownerOfDiagram="_p">
          <ownedDiagram xmi:type="uml:Diagram" xmi:id="_inner" name="Inner" ownerOfDiagram="_a">
            <xmi:Extension extender="Example UML Tool 1.0">
              <diagramRepresentation>
                <diagram:DiagramRepresentationObject type="SysML Internal Block Diagram" umlType="Composite Structure Diagram">
                  <diagramContents><usedElements>_a_b</usedElements></diagramContents>
                </diagram:DiagramRepresentationObject>
              </diagramRepresentation>
            </xmi:Extension>
          </ownedDiagram>
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="SysML Package Diagram" umlType="Class Diagram">
                <diagramContents><usedElements>_a</usedElements></diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>`)
	if len(m.Diagrams) != 2 {
		t.Fatalf("diagrams = %+v", m.Diagrams)
	}
	outer, inner := m.Diagram("_outer"), m.Diagram("_inner")
	if outer.Kind != "SysML Package Diagram" || ids(outer.Shown) != "_a" {
		t.Errorf("outer = %+v", outer)
	}
	if inner.Kind != "SysML Internal Block Diagram" || ids(inner.Shown) != "_a_b" {
		t.Errorf("inner = %+v", inner)
	}
	if m.Diagram("#_inner") != inner || m.Diagram("model.xml#_outer") != outer {
		t.Error("an href's fragment does not find its diagram")
	}
	if m.Diagram("_gone") != nil || m.Diagram("model.xml#") != nil || m.Diagram("") != nil {
		t.Error("an id no diagram has finds one")
	}
}

func TestDiagramsInArchiveEntriesResolveAcrossDocuments(t *testing.T) {
	main := diagramDocument(`
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Across" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="SysML Package Diagram" umlType="Class Diagram">
                <diagramContents><usedElements>_shared</usedElements></diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>`)
	shared := []byte(`<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Package xmi:type="uml:Package" xmi:id="_shared" name="Shared"/>
</xmi:XMI>`)
	m, err := Parse(archive(t, "", map[string][]byte{"main.xml": main, "shared.xml": shared}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Diagrams) != 1 || m.Diagrams[0].Shown[0].Element != m.Lookup("_shared") {
		t.Fatalf("diagrams = %+v", m.Diagrams)
	}
}

func TestBareModuleFragmentResolvesOnlyWhenUnique(t *testing.T) {
	// A tool that referenced a module element by href goes on to write its
	// bare id, which resolves to that href's proxy while one document holds
	// the fragment; once hrefs of two documents share it, the id is ambiguous
	// and names no element rather than the first proxy seen.
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:diagram="http://www.example.com/tool/diagram">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_x" name="x">
        <type xmi:type="uml:Class" href="module-a.xmi#_type1"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_y" name="y">
        <type xmi:type="uml:Class" href="module-b.xmi#_type1"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_z" name="z">
        <type xmi:type="uml:Class" href="module-a.xmi#_only"/>
      </ownedAttribute>
    </packagedElement>
    <xmi:Extension extender="Example UML Tool 1.0">
      <modelExtension>
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Modules" ownerOfDiagram="_m">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="SysML Package Diagram" umlType="Class Diagram">
                <diagramContents>
                  <usedElements>_type1</usedElements>
                  <usedElements>_only</usedElements>
                  <usedObjects href="module-b.xmi#_type1"/>
                </diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>
      </modelExtension>
    </xmi:Extension>
  </uml:Model>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	d := m.Diagrams[0]
	if got := ids(d.Shown); got != "_type1? _only module-b.xmi#_type1" {
		t.Errorf("shown = %q", got)
	}
	if d.Shown[1].Element != m.Lookup("module-a.xmi#_only") || d.Shown[2].Element != m.Lookup("module-b.xmi#_type1") {
		t.Errorf("shown elements = %+v", d.Shown)
	}
	if got := strings.Join(m.Ambiguous("_type1"), " "); got != "module-a.xmi#_type1 module-b.xmi#_type1" {
		t.Errorf("Ambiguous(_type1) = %q", got)
	}
	if m.Ambiguous("_only") != nil || m.Ambiguous("module-a.xmi#_type1") != nil || m.Ambiguous("_a") != nil || m.Ambiguous("_gone") != nil {
		t.Error("a resolved or unknown id reads as ambiguous")
	}
}
