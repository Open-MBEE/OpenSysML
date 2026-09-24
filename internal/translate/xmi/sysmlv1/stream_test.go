package sysmlv1

import (
	"archive/zip"
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// A stream is read the way MagicDraw writes it: unbound xmi: prefixes, the
// frame naming the diagram, symbols nested to any depth naming their elements
// once each, and top-level symbols naming none counted as free by class.
func TestReadSymbols(t *testing.T) {
	stream := `<?xml version="1.0" encoding="UTF-8"?>
<mdOwnedViews>
  <mdElement elementClass="DiagramFrame" xmi:id="_frame">
    <elementID xmi:idref="_diag"/>
    <geometry>10, 10, 800, 600</geometry>
  </mdElement>
  <mdElement elementClass="Class" xmi:id="_s1">
    <elementID xmi:idref="_a"/>
    <mdOwnedViews>
      <mdElement elementClass="Part" xmi:id="_s2">
        <elementID xmi:idref="_a_b"/>
        <mdOwnedViews>
          <mdElement elementClass="Pin" xmi:id="_s3">
            <elementID xmi:idref="_pin"/>
          </mdElement>
          <mdElement elementClass="TextBox" xmi:id="_s4"/>
        </mdOwnedViews>
      </mdElement>
    </mdOwnedViews>
  </mdElement>
  <mdElement elementClass="Class" xmi:id="_s5">
    <elementID xmi:idref="_a"/>
  </mdElement>
  <mdElement elementClass="Generalization" xmi:id="_s6">
    <elementID href="other.xmi#_gen"/>
  </mdElement>
  <mdElement elementClass="ImageShape" xmi:id="_s7">
    <elementID/>
  </mdElement>
  <mdElement elementClass="TextBox" xmi:id="_s8"/>
  <mdElement elementClass="TextBox" xmi:id="_s9"/>
</mdOwnedViews>`
	syms, err := readSymbols([]byte(stream), "_diag")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"_a", "_a_b", "_pin", "other.xmi#_gen"}; !reflect.DeepEqual(syms.shown, want) {
		t.Errorf("shown = %q, want %q", syms.shown, want)
	}
	if want := map[string]int{"ImageShape": 1, "TextBox": 2}; !reflect.DeepEqual(syms.free, want) {
		t.Errorf("free = %v, want %v", syms.free, want)
	}
}

func TestReadSymbolsOfEmptyStream(t *testing.T) {
	for name, stream := range map[string]string{
		"bare":  `<mdOwnedViews/>`,
		"frame": `<mdOwnedViews><mdElement elementClass="DiagramFrame"><elementID xmi:idref="_diag"/></mdElement></mdOwnedViews>`,
	} {
		syms, err := readSymbols([]byte(stream), "_diag")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(syms.shown) != 0 || len(syms.free) != 0 {
			t.Errorf("%s: shown %q, free %v; want none", name, syms.shown, syms.free)
		}
	}
}

func TestReadSymbolsRejectsOtherContent(t *testing.T) {
	for name, stream := range map[string]string{
		"empty":     "",
		"binary":    "\xff\xd8\xff\xe0 not xml",
		"other xml": `<?xml version="1.0"?><options><option name="grid"/></options>`,
	} {
		if _, err := readSymbols([]byte(stream), "_diag"); err == nil {
			t.Errorf("%s: read as symbols", name)
		}
	}
}

// A stream cut short is not read as a partial diagram: the symbols after the
// cut are unknown, so the whole is unreadable.
func TestReadSymbolsRejectsTruncatedStream(t *testing.T) {
	whole := `<mdOwnedViews><mdElement elementClass="Class" xmi:id="_s1"><elementID xmi:idref="_a"/><mdOwnedViews><mdElement elementClass="Part" xmi:id="_s2"><elementID xmi:idref="_a_b"/></mdElement></mdOwnedViews></mdElement><mdElement elementClass="TextBox" xmi:id="_s3"/></mdOwnedViews>`
	if _, err := readSymbols([]byte(whole), "_diag"); err != nil {
		t.Fatalf("the whole stream: %v", err)
	}
	for name, stream := range map[string]string{
		"root open":       `<mdOwnedViews>`,
		"after a symbol":  whole[:strings.Index(whole, `<mdElement elementClass="TextBox"`)],
		"inside a symbol": whole[:strings.Index(whole, `</mdOwnedViews></mdElement>`)],
		"inside a tag":    whole[:len(whole)-3],
		"mismatched end":  strings.Replace(whole, `</mdElement><mdElement elementClass="TextBox"`, `</mdOwnedViews><mdElement elementClass="TextBox"`, 1),
	} {
		if syms, err := readSymbols([]byte(stream), "_diag"); err == nil {
			t.Errorf("%s: read as symbols %q", name, syms.shown)
		}
	}
}

// Every diagram is read from its stream in the archive: one whose tool lists
// no used element gets what the symbols draw, one whose list names elements
// keeps the list in order and gains the elements the symbols draw beyond it,
// however the two spell an element's href, while hrefs into two modules stay
// two elements though their fragments agree; one whose stream is absent or
// unreadable stays unread with its list as written.
func TestParseArchiveReadsDiagramStreams(t *testing.T) {
	model := strings.Replace(string(diagramDocument(bddDiagram+`
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d_empty" name="Empty" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject xmi:id="_d_empty_rep" type="SysML Block Definition Diagram" umlType="Class Diagram">
                <diagramContents xmi:id="_d_empty_contents">
                  <binaryObject xsi:type="binary:StreamIdentityBinaryObject" streamContentID="BINARY-empty"/>
                </diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d_drawn" name="Drawn" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject xmi:id="_d_drawn_rep" type="SysML Block Definition Diagram" umlType="Class Diagram">
                <diagramContents xmi:id="_d_drawn_contents">
                  <binaryObject xsi:type="binary:StreamIdentityBinaryObject" streamContentID="BINARY-drawn"/>
                </diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d_lost" name="Lost" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject xmi:id="_d_lost_rep" type="SysML Block Definition Diagram" umlType="Class Diagram">
                <diagramContents xmi:id="_d_lost_contents">
                  <binaryObject xsi:type="binary:StreamIdentityBinaryObject" streamContentID="BINARY-lost"/>
                </diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d_cut" name="Cut" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject xmi:id="_d_cut_rep" type="SysML Block Definition Diagram" umlType="Class Diagram">
                <diagramContents xmi:id="_d_cut_contents">
                  <binaryObject xsi:type="binary:StreamIdentityBinaryObject" streamContentID="BINARY-cut"/>
                </diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>`)),
		`xmlns:diagram=`, `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:diagram=`, 1)
	model = strings.Replace(model, `<packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>`,
		`<packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_b_x" name="x"><type xmi:type="uml:Class" href="ModuleA.xmi#_shared"/></ownedAttribute>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_b_y" name="y"><type xmi:type="uml:Class" href="ModuleB.xmi#_shared"/></ownedAttribute>
      </packagedElement>`, 1)
	m, err := Parse(archive(t, "", map[string][]byte{
		"com.nomagic.magicdraw.uml_model.model": []byte(model),
		"BINARY-1": []byte(`<mdOwnedViews><mdElement elementClass="Class"><elementID xmi:idref="_b"/></mdElement>` +
			`<mdElement elementClass="DataType"><elementID href="PrimitiveTypes.mdzip#Real"/></mdElement>` +
			`<mdElement elementClass="Class"><elementID xmi:idref="_a"/><mdOwnedViews><mdElement elementClass="Part"><elementID xmi:idref="_a_m"/></mdElement></mdOwnedViews></mdElement>` +
			`<mdElement elementClass="TextBox"/></mdOwnedViews>`),
		"BINARY-empty": []byte(`<mdOwnedViews><mdElement elementClass="DiagramFrame"><elementID xmi:idref="_d_empty"/></mdElement><mdElement elementClass="ImageShape"/></mdOwnedViews>`),
		"BINARY-drawn": []byte(`<mdOwnedViews><mdElement elementClass="Class"><elementID xmi:idref="_b"/><mdOwnedViews><mdElement elementClass="Part"><elementID xmi:idref="_a_b"/></mdElement></mdOwnedViews></mdElement>` +
			`<mdElement elementClass="DataType"><elementID href="PrimitiveTypes.mdzip#Real"/></mdElement>` +
			`<mdElement elementClass="Class"><elementID href="ModuleA.xmi#_shared"/></mdElement>` +
			`<mdElement elementClass="Class"><elementID href="ModuleB.xmi#_shared"/></mdElement></mdOwnedViews>`),
		"BINARY-lost": []byte("\xff\xfe not a stream"),
		"BINARY-cut":  []byte(`<mdOwnedViews><mdElement elementClass="Class"><elementID xmi:idref="_b"/></mdElement><mdElement elementClass="Class"><elementID xmi:idref="_a"/>`),
	}))
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*Diagram{}
	for i := range m.Diagrams {
		byID[m.Diagrams[i].ID] = &m.Diagrams[i]
	}
	shown := func(d *Diagram) []string {
		var ids []string
		for _, r := range d.Shown {
			ids = append(ids, r.ID)
		}
		return ids
	}
	if d := byID["_d_bdd"]; !d.Drawn || len(d.Shown) != 7 || shown(d)[0] != "_a" || shown(d)[6] != "_a_m" ||
		d.Shown[6].Element != m.Lookup("_a_m") || !reflect.DeepEqual(d.Free, map[string]int{"TextBox": 1}) {
		t.Errorf("listed diagram: drawn %v, shown %q, free %v; want the list kept and the unlisted element after it", d.Drawn, shown(d), d.Free)
	}
	if d := byID["_d_empty"]; !d.Drawn || len(d.Shown) != 0 || !reflect.DeepEqual(d.Free, map[string]int{"ImageShape": 1}) {
		t.Errorf("empty diagram: drawn %v, shown %q, free %v", d.Drawn, shown(d), d.Free)
	}
	if d := byID["_d_drawn"]; !d.Drawn || !reflect.DeepEqual(shown(d), []string{"_b", "_a_b", "PrimitiveTypes.mdzip#Real", "ModuleA.xmi#_shared", "ModuleB.xmi#_shared"}) || len(d.Free) != 0 ||
		d.Shown[0].Element == nil || d.Shown[1].Element == nil ||
		d.Shown[2].Element != m.Lookup("http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real") ||
		d.Shown[3].Element != m.Lookup("ModuleA.xmi#_shared") || d.Shown[4].Element != m.Lookup("ModuleB.xmi#_shared") || d.Shown[3].Element == d.Shown[4].Element {
		t.Errorf("drawn diagram: drawn %v, shown %q, free %v; want the module-file href resolved to the proxy and the two modules' elements both shown", d.Drawn, shown(d), d.Free)
	}
	if d := byID["_d_lost"]; d.Drawn || len(d.Shown) != 0 || !d.Represented() {
		t.Errorf("unreadable stream: drawn %v, shown %q, represented %v", d.Drawn, shown(d), d.Represented())
	}
	if d := byID["_d_cut"]; d.Drawn || len(d.Shown) != 0 || !d.Represented() {
		t.Errorf("truncated stream: drawn %v, shown %q, represented %v; want the diagram unread, not read in part", d.Drawn, shown(d), d.Represented())
	}
}

// A stream entry that cannot be extracted leaves its diagram unread, as an
// undecodable one does; the model it is presentation for is still parsed.
func TestParseArchiveSurvivesTornDiagramStream(t *testing.T) {
	model := strings.Replace(string(diagramDocument(bddDiagram+`
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d_torn" name="Torn" ownerOfDiagram="_p">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject xmi:id="_d_torn_rep" type="SysML Block Definition Diagram" umlType="Class Diagram">
                <diagramContents xmi:id="_d_torn_contents">
                  <binaryObject xsi:type="binary:StreamIdentityBinaryObject" streamContentID="BINARY-torn"/>
                </diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>`)),
		`xmlns:diagram=`, `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:diagram=`, 1)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("com.nomagic.magicdraw.uml_model.model")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(model))
	raw, err := zw.CreateRaw(&zip.FileHeader{Name: "BINARY-torn", Method: zip.Deflate, UncompressedSize64: 64, CompressedSize64: 8})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = raw.Write([]byte("not defl"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	m, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("a torn diagram stream failed the model: %v", err)
	}
	var torn *Diagram
	for i := range m.Diagrams {
		if m.Diagrams[i].ID == "_d_torn" {
			torn = &m.Diagrams[i]
		}
	}
	if torn == nil || torn.Drawn || len(torn.Shown) != 0 || !torn.Represented() {
		t.Errorf("torn stream: %+v; want an unread, represented diagram", torn)
	}
	if m.Lookup("_b") == nil {
		t.Error("the model beside the torn stream was not read")
	}
}
