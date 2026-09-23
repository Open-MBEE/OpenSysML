package mtip

import (
	"strings"
	"testing"
)

// huds wraps <data> records in the packet shape an MTIP export writes.
func huds(records ...string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><packet>
<metadata><mtipVersion>2022x v1.0.0</mtipVersion><cameoVersion>2022x Refresh2</cameoVersion><exportTime>2025-06-12T21:38:50Z</exportTime></metadata>
` + strings.Join(records, "\n") + `
</packet>`
}

func dataRecord(id, typ, relationships string) string {
	return `<data>
  <attributes _dtype="dict">
    <attribute _dtype="dict" key="name"><attribute _dtype="str" key="value">` + "NAMEMARKER" + `</attribute></attribute>
  </attributes>
  <id _dtype="dict"><cameo _dtype="str">` + id + `</cameo></id>
  <relationships _dtype="dict">` + relationships + `</relationships>
  <type _dtype="str">` + typ + `</type>
</data>`
}

func named(name, record string) string {
	return strings.Replace(record, "NAMEMARKER", name, 1)
}

func elementList(items ...string) string {
	return `<element _dtype="list">` + strings.Join(items, "") + `</element>`
}

func connectorList(items ...string) string {
	return `<diagramConnector _dtype="list">` + strings.Join(items, "") + `</diagramConnector>`
}

func placement(key, id, typ, meta string) string {
	return `<element _dtype="dict" key="` + key + `"><relationship_metadata _dtype="dict">` + meta +
		`</relationship_metadata><id _dtype="str">` + id + `</id><type _dtype="str">` + typ + `</type></element>`
}

func bounds(top, bottom, left, right string) string {
	return `<top _dtype="int">` + top + `</top><bottom _dtype="int">` + bottom + `</bottom><left _dtype="int">` + left + `</left><right _dtype="int">` + right + `</right>`
}

func pt(x, y string) string {
	return `<xCoordinate _dtype="int">` + x + `</xCoordinate><yCoordinate _dtype="int">` + y + `</yCoordinate>`
}

func connector(key, id, typ, meta string) string {
	return `<diagramConnector _dtype="dict" key="` + key + `"><relationship_metadata _dtype="dict">` + meta +
		`</relationship_metadata><id _dtype="str">` + id + `</id><type _dtype="str">` + typ + `</type></diagramConnector>`
}

func parse(t *testing.T, src string) *Export {
	t.Helper()
	ex, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return ex
}

func TestReadsMetadataAndDiagram(t *testing.T) {
	ex := parse(t, huds(
		dataRecord("_m1", "sysml.Package", ""),
		named("Machine", dataRecord("_d1", "sysml.StateMachineDiagram",
			elementList(placement("0", "_e1", "sysml.State", bounds("-10", "-40", "5", "25")))+
				connectorList(connector("0", "_c1", "sysml.Transition",
					`<supplierPoint _dtype="dict">`+pt("70", "84")+`</supplierPoint>`+
						`<clientPoint _dtype="dict">`+pt("71", "46")+`</clientPoint>`+
						`<breakPoint _dtype="list"/>`))))),
	)
	if ex.MTIPVersion != "2022x v1.0.0" || ex.CameoVersion != "2022x Refresh2" || ex.ExportTime != "2025-06-12T21:38:50Z" {
		t.Errorf("metadata: %q %q %q", ex.MTIPVersion, ex.CameoVersion, ex.ExportTime)
	}
	if ex.Records != 2 {
		t.Errorf("records: %d", ex.Records)
	}
	if len(ex.Diagrams) != 1 {
		t.Fatalf("diagrams: %d", len(ex.Diagrams))
	}
	d := ex.Diagrams[0]
	if d.ID != "_d1" || d.Type != "sysml.StateMachineDiagram" || d.Name != "Machine" {
		t.Errorf("diagram: %q %q %q", d.ID, d.Type, d.Name)
	}
	p := d.Placements[0]
	if p.ID != "_e1" || p.Type != "sysml.State" || p.X != 5 || p.Y != 10 || p.Width != 20 || p.Height != 30 {
		t.Errorf("placement: %+v", p)
	}
	c := d.Connectors[0]
	if c.ID != "_c1" || len(c.Points) != 4 || c.Points[0] != 71 || c.Points[1] != 46 || c.Points[2] != 70 || c.Points[3] != 84 {
		t.Errorf("connector: %+v", c)
	}
}

func TestRouteOrdersClientBreaksReversedSupplier(t *testing.T) {
	breaks := `<breakPoint _dtype="list">` +
		`<breakPoint key="0">` + pt("371", "721") + `</breakPoint>` +
		`<breakPoint key="1">` + pt("648", "721") + `</breakPoint>` +
		`</breakPoint>`
	ex := parse(t, huds(named("D", dataRecord("_d", "sysml.BlockDefinitionDiagram",
		connectorList(connector("0", "_c", "sysml.Dependency",
			`<supplierPoint _dtype="dict">`+pt("648", "704")+`</supplierPoint>`+
				`<clientPoint _dtype="dict">`+pt("371", "704")+`</clientPoint>`+breaks))))))
	got := ex.Diagrams[0].Connectors[0].Points
	want := []float64{371, 704, 648, 721, 371, 721, 648, 704}
	if len(got) != len(want) {
		t.Fatalf("points %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("points %v, want %v", got, want)
		}
	}
}

func TestKeyOrderAndEmptyDiagram(t *testing.T) {
	ex := parse(t, huds(
		named("Empty", dataRecord("_d0", "sysml.ActivityDiagram", elementList())),
		named("Ordered", dataRecord("_d1", "sysml.BlockDefinitionDiagram",
			elementList(
				placement("1", "_e1", "sysml.Class", bounds("-1", "-9", "50", "60")),
				placement("0", "_e0", "sysml.Class", bounds("-1", "-9", "0", "10")),
			))),
	))
	if len(ex.Diagrams) != 2 {
		t.Fatalf("diagrams: %d", len(ex.Diagrams))
	}
	if len(ex.Diagrams[0].Placements) != 0 || len(ex.Diagrams[0].Malformed) != 0 {
		t.Errorf("the empty element list is a valid record: %+v", ex.Diagrams[0])
	}
	if ex.Diagrams[1].Placements[0].ID != "_e0" || ex.Diagrams[1].Placements[1].ID != "_e1" {
		t.Errorf("placements not in key order: %+v", ex.Diagrams[1].Placements)
	}
}

func TestNonDiagramRecordsIgnored(t *testing.T) {
	ex := parse(t, huds(
		dataRecord("_m1", "sysml.Model", `<hasParent _dtype="dict"><type _dtype="str">sysml.Model</type></hasParent>`),
		dataRecord("_m2", "sysml.Block", `<relationship_metadata _dtype="dict"/>`),
	))
	if ex.Records != 2 {
		t.Errorf("records: %d", ex.Records)
	}
	if len(ex.Diagrams) != 0 {
		t.Errorf("non-diagram records were kept: %+v", ex.Diagrams)
	}
}

func TestMalformedRecordedNotFatal(t *testing.T) {
	ex := parse(t, huds(named("D", dataRecord("_d", "sysml.BlockDefinitionDiagram",
		elementList(
			placement("0", "_e1", "sysml.Class", `<top _dtype="int">-1</top><left _dtype="int">0</left><right _dtype="int">9</right>`),
			placement("1", "", "sysml.Class", bounds("-1", "-9", "0", "9")),
			placement("2", "_e2", "sysml.Class", `<top _dtype="int">abc</top><bottom _dtype="int">-9</bottom><left _dtype="int">0</left><right _dtype="int">9</right>`),
			placement("3", "_ok", "sysml.Class", bounds("-1", "-9", "0", "9")),
		)+connectorList(
			connector("0", "_c0", "sysml.Dependency", `<clientPoint _dtype="dict">`+pt("1", "2")+`</clientPoint>`),
			connector("1", "_c1", "sysml.Dependency", `<supplierPoint _dtype="dict">`+pt("1", "2")+`</supplierPoint><clientPoint _dtype="dict">`+pt("3", "4")+`</clientPoint>`),
			connector("2", "_c2", "sysml.Dependency", `<supplierPoint _dtype="dict">`+pt("1", "2")+`</supplierPoint><clientPoint _dtype="dict">`+pt("3", "4")+`</clientPoint><breakPoint _dtype="list"><breakPoint key="0">`+pt("5", "6")+`</breakPoint><breakPoint key="1"><xCoordinate _dtype="int">7</xCoordinate></breakPoint></breakPoint>`),
		)))))
	d := ex.Diagrams[0]
	if len(d.Placements) != 1 || d.Placements[0].ID != "_ok" {
		t.Errorf("placements: %+v", d.Placements)
	}
	if len(d.Connectors) != 1 || d.Connectors[0].ID != "_c1" {
		t.Errorf("connectors: %+v", d.Connectors)
	}
	if len(d.Malformed) != 5 {
		t.Fatalf("malformed: %v", d.Malformed)
	}
	for _, want := range []string{"_e1: no <bottom> bound", "placement #1: no <id>", `_e2: a non-numeric <top> bound`, "_c0: no <supplierPoint> or <clientPoint>", "_c2: a <breakPoint> is missing a coordinate"} {
		found := false
		for _, m := range d.Malformed {
			if strings.Contains(m, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no malformed entry naming %q in %v", want, d.Malformed)
		}
	}
}

func TestUnsupportedTagsCounted(t *testing.T) {
	ex := parse(t, huds(named("D", dataRecord("_d", "sysml.BlockDefinitionDiagram",
		elementList(placement("0", "_e1", "sysml.Class",
			bounds("-1", "-9", "0", "9")+
				`<fillColor _dtype="str">#ff0000</fillColor><fillColor _dtype="str">#00ff00</fillColor><font _dtype="dict"><size>12</size></font>`))+
			connectorList(connector("0", "_c1", "sysml.Dependency",
				`<supplierPoint _dtype="dict">`+pt("1", "2")+`</supplierPoint><clientPoint _dtype="dict">`+pt("3", "4")+`</clientPoint><lineStyle _dtype="str">dashed</lineStyle>`))))))
	u := ex.Diagrams[0].Unsupported
	if u["fillColor"] != 2 || u["font"] != 1 || u["lineStyle"] != 1 {
		t.Errorf("unsupported: %v", u)
	}
	if len(ex.Diagrams[0].Placements) != 1 || len(ex.Diagrams[0].Connectors) != 1 {
		t.Errorf("an unsupported property must not drop the record")
	}
}

func TestNameAbsent(t *testing.T) {
	ex := parse(t, huds(`<data>
  <id _dtype="dict"><cameo _dtype="str">_d</cameo></id>
  <relationships _dtype="dict">`+elementList()+`</relationships>
  <type _dtype="str">sysml.ActivityDiagram</type>
</data>`))
	if ex.Diagrams[0].Name != "" {
		t.Errorf("name %q", ex.Diagrams[0].Name)
	}
}

func TestNotWellFormedFails(t *testing.T) {
	if _, err := Parse([]byte("<packet><data><id></packet>")); err == nil {
		t.Error("expected an error for malformed XML")
	}
	if _, err := Parse([]byte("<packet><data>")); err == nil {
		t.Error("expected an error for truncated XML")
	}
}

// A diagram record with no <id> keeps its geometry and records the missing id.
func TestDiagramRecordWithoutID(t *testing.T) {
	rec := `<data>
  <relationships _dtype="dict">` + elementList(placement("0", "_e1", "sysml.State", bounds("-10", "-40", "5", "25"))) + `</relationships>
  <type _dtype="str">sysml.StateMachineDiagram</type>
</data>`
	ex := parse(t, huds(rec))
	if len(ex.Diagrams) != 1 {
		t.Fatalf("diagrams: %d", len(ex.Diagrams))
	}
	d := ex.Diagrams[0]
	if d.ID != "" {
		t.Errorf("id: %q", d.ID)
	}
	if len(d.Malformed) != 1 || d.Malformed[0] != "the diagram record has no <id>" {
		t.Errorf("malformed: %v", d.Malformed)
	}
	if len(d.Placements) != 1 || d.Placements[0].ID != "_e1" {
		t.Errorf("placements: %+v", d.Placements)
	}
}

// A bound or coordinate that parses but is not finite is malformed, not
// written out as an invalid token.
func TestNonFiniteNumbersAreMalformed(t *testing.T) {
	for _, bad := range []string{"NaN", "+Inf", "-Inf"} {
		ex := parse(t, huds(named("D", dataRecord("_d", "sysml.BlockDefinitionDiagram",
			elementList(placement("0", "_e1", "sysml.Block", bounds(bad, "-40", "5", "25")))))))
		d := ex.Diagrams[0]
		if len(d.Placements) != 0 {
			t.Errorf("%s: placements %+v", bad, d.Placements)
		}
		if len(d.Malformed) != 1 {
			t.Errorf("%s: malformed %v", bad, d.Malformed)
		}
	}
}

// Only a HUDS <packet> is an MTIP export: another root is refused outright,
// and a real packet with metadata and no diagram records is a valid export.
func TestNonPacketRootRefused(t *testing.T) {
	for _, src := range []string{`<html/>`, `<export><data/></export>`} {
		_, err := Parse([]byte(src))
		if err == nil {
			t.Fatalf("Parse(%q): no error", src)
		}
	}
	if _, err := Parse([]byte(`<html/>`)); err.Error() != "the MTIP export's root element is <html>, not <packet>" {
		t.Errorf("error: %v", err)
	}
	ex := parse(t, `<packet><metadata><mtipVersion>2022x</mtipVersion></metadata></packet>`)
	if len(ex.Diagrams) != 0 || ex.Records != 0 || ex.MTIPVersion != "2022x" {
		t.Errorf("empty packet: %+v", ex)
	}
}

// A document with no root element at all is refused.
func TestEmptyDocumentRefused(t *testing.T) {
	if _, err := Parse([]byte(`<?xml version="1.0"?><!-- nothing -->`)); err == nil ||
		err.Error() != "the MTIP export is empty" {
		t.Errorf("error: %v", err)
	}
}
