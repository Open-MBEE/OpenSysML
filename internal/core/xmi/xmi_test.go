package xmi

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"
)

const fixture = "../migrate/testdata/xmi/vehicle.xmi"

func readFixture(t *testing.T) *Model {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestParseTree(t *testing.T) {
	m := readFixture(t)
	if m.Exporter != "Example UML Tool" {
		t.Errorf("exporter = %q", m.Exporter)
	}
	if len(m.Roots) != 2 || m.Roots[0].Type != "Model" || m.Roots[0].Name != "Model" {
		t.Fatalf("roots = %+v", m.Roots)
	}
	vehicle := m.Lookup("_blk_vehicle")
	if vehicle == nil || vehicle.Type != "Class" || vehicle.Role != "packagedElement" {
		t.Fatalf("vehicle = %+v", vehicle)
	}
	if got := strings.Join(vehicle.Path(), "::"); got != "Model::Vehicle Design::Vehicle" {
		t.Errorf("path = %q", got)
	}
	if attrs := vehicle.Owned("ownedAttribute"); len(attrs) != 13 {
		t.Errorf("owned attributes = %d", len(attrs))
	}
	// The diagram inside xmi:Extension is tool-private and not read.
	if m.Lookup("_diag_bdd") != nil {
		t.Error("xmi:Extension content was read")
	}
}

func TestReferences(t *testing.T) {
	m := readFixture(t)
	engine := m.Lookup("_prop_engine")
	if got := m.Ref(engine, "type"); got == nil || got.ID != "_blk_engine" {
		t.Errorf("type ref = %+v", got)
	}
	// Multi-valued attribute references are space-separated.
	assoc := m.Lookup("_assoc_vehicle_engine")
	if ends := m.Refs(assoc, "memberEnd"); len(ends) != 2 || ends[1].ID != "_ae_vehicle_1" {
		t.Errorf("memberEnd = %+v", ends)
	}
	// Child idref elements resolve too.
	cmt := m.Lookup("_cmt_vehicle")
	if got := m.Ref(cmt, "annotatedElement"); got == nil || got.ID != "_blk_vehicle" {
		t.Errorf("annotatedElement = %+v", got)
	}
	// An href yields a named proxy when its fragment reads as a name.
	name := m.Lookup("_prop_name")
	typ := m.Ref(name, "type")
	if typ == nil || !typ.IsProxy() || typ.Name != "String" {
		t.Errorf("href type = %+v", typ)
	}
	// An opaque body is the element's text.
	body := m.Lookup("_dv_total").Owned("body")
	if len(body) != 1 || strings.TrimSpace(body[0].Text) != "mass + engine.mass" {
		t.Errorf("body = %+v", body)
	}
}

func TestStereotypes(t *testing.T) {
	m := readFixture(t)
	vehicle := m.Lookup("_blk_vehicle")
	block := vehicle.Stereotype("Block")
	if block == nil || block.Tag("isEncapsulated") != "true" {
		t.Fatalf("Block = %+v", block)
	}
	if !strings.Contains(block.Namespace, "SysML") {
		t.Errorf("namespace = %q", block.Namespace)
	}
	req := m.Lookup("_req_mass_1").Stereotype("Requirement")
	if req == nil || req.Tag("id") != "R1.1" || req.Tag("text") != "The chassis shall have a mass of less than 400 kg." {
		t.Errorf("Requirement tags = %+v", req)
	}
	nested := m.Lookup("_ce_nested_2").Stereotype("NestedConnectorEnd")
	if nested == nil || len(nested.Tags["propertyPath"]) != 2 || nested.Tags["propertyPath"][1] != "_prop_piston" {
		t.Errorf("propertyPath = %+v", nested)
	}
	critical := m.Lookup("_blk_engine").Stereotype("Critical")
	if critical == nil || critical.Tag("level") != "high" || !strings.Contains(critical.Namespace, "example.com") {
		t.Errorf("user profile stereotype = %+v", critical)
	}
}

// archive zips the entries after a prefix, as a self-extracting stub is.
func archive(t *testing.T, prefix string, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString(prefix)
	zw := zip.NewWriter(&buf)
	zw.SetOffset(int64(len(prefix)))
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(content)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseArchive(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	project := map[string][]byte{
		"PROJECT_MANIFEST":                           []byte("<options/>"),
		"com.nomagic.magicdraw.core.project.options": []byte("<options/>"),
		"com.nomagic.magicdraw.uml_model.model":      data,
	}
	for name, in := range map[string][]byte{
		"project":       archive(t, "", project),
		"stub-prefixed": archive(t, "#!/bin/sh\nexit 0\n", project),
	} {
		m, err := Parse(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if m.Lookup("_blk_vehicle") == nil {
			t.Errorf("%s: archive model entry was not read", name)
		}
	}
}

func TestParseArchiveWithoutModel(t *testing.T) {
	_, err := Parse(archive(t, "", map[string][]byte{"readme.txt": []byte("nothing")}))
	if err == nil || !strings.Contains(err.Error(), "readme.txt") {
		t.Errorf("err = %v", err)
	}
	_, err = Parse(archive(t, "", nil))
	if err == nil || !strings.Contains(err.Error(), "archive holds no model") {
		t.Errorf("empty: err = %v", err)
	}
}

func TestParseRejectsTruncatedArchive(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	in := archive(t, "", map[string][]byte{"model.xmi": data})
	_, err = Parse(in[:len(in)/2])
	if err == nil || !strings.Contains(err.Error(), "reading archive") {
		t.Errorf("err = %v", err)
	}
}

func TestParseRejectsNonXMI(t *testing.T) {
	for _, in := range []string{"part def V;", "<html><body/></html>", ""} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%q: accepted", in)
		}
	}
}

func TestParseBareModelRoot(t *testing.T) {
	src := `<?xml version="1.0"?>
<uml:Model xmi:version="2.1" xmlns:xmi="http://schema.omg.org/spec/XMI/2.1" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="M">
  <packagedElement xmi:type="uml:Class" xmi:id="c" name="C"/>
</uml:Model>`
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Roots) != 1 || m.Roots[0].Type != "Model" || m.Lookup("c") == nil {
		t.Errorf("roots = %+v", m.Roots)
	}
}

const wrapperOnly = `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001">
  <xmi:Documentation exporter="Example UML Tool"/>
</xmi:XMI>`

func TestParseArchiveIgnoresUnrelatedXML(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	metadata := []byte(`<?xml version="1.0"?><project><option name="x">1</option></project>`)
	t.Run("project entry", func(t *testing.T) {
		m, err := Parse(archive(t, "", map[string][]byte{
			"com.nomagic.magicdraw.uml_model.model": data,
			"metadata/settings.xml":                 metadata,
			"broken.xmi":                            []byte("<xmi:XMI"),
		}))
		if err != nil {
			t.Fatal(err)
		}
		if m.Lookup("_blk_vehicle") == nil {
			t.Error("project model entry was not read")
		}
	})
	t.Run("xmi fallback", func(t *testing.T) {
		m, err := Parse(archive(t, "", map[string][]byte{
			"model.xmi":    data,
			"settings.xml": metadata,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if m.Lookup("_blk_vehicle") == nil {
			t.Error(".xmi entry was not read")
		}
	})
	t.Run("malformed project entry", func(t *testing.T) {
		_, err := Parse(archive(t, "", map[string][]byte{
			"com.nomagic.magicdraw.uml_model.model": append(data[:len(data)/2:len(data)/2], []byte("<broken")...),
		}))
		if err == nil || !strings.Contains(err.Error(), "uml_model.model") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("malformed xmi fallback", func(t *testing.T) {
		_, err := Parse(archive(t, "", map[string][]byte{
			"model.xmi": data[:len(data)/2],
		}))
		if err == nil || !strings.Contains(err.Error(), "model.xmi") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestParseRejectsWrapperWithoutModel(t *testing.T) {
	if _, err := Parse([]byte(wrapperOnly)); err == nil || !strings.Contains(err.Error(), "no model") {
		t.Errorf("direct: err = %v", err)
	}
	_, err := Parse(archive(t, "", map[string][]byte{"com.nomagic.magicdraw.uml_model.model": []byte(wrapperOnly)}))
	if err == nil || !strings.Contains(err.Error(), "no model") {
		t.Errorf("archive: err = %v", err)
	}
}

func TestParseRejectsTruncatedDocument(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data[:len(data)/2]); err == nil {
		t.Error("truncated document accepted")
	}
}

func TestParseRejectsMalformedXML(t *testing.T) {
	for _, in := range []string{
		`<xmi:XMI xmlns:xmi="x" xmlns:uml="u"><uml:Model xmi:id="_m" name="M"><packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"></uml:Model></xmi:XMI>`,
		`<xmi:XMI xmlns:xmi="x" xmlns:uml="u"><uml:Model xmi:id="_m" name="A &nbsp; B"/></xmi:XMI>`,
	} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%q: accepted", in)
		}
	}
}

func TestUnresolvedReferences(t *testing.T) {
	m := readFixture(t)
	assoc := m.Lookup("_assoc_vehicle_engine")
	if got := m.Unresolved(assoc, "memberEnd"); len(got) != 0 {
		t.Errorf("Unresolved = %v", got)
	}
	e := &Element{Attrs: map[string]string{"client": "_prop_engine _nowhere"}}
	if got := m.Unresolved(e, "client"); len(got) != 1 || got[0] != "_nowhere" {
		t.Errorf("Unresolved = %v", got)
	}
}

// A child carrying href or xmi:idref is a reference however it is typed: the
// xmi:type describes the target, which the proxy keeps.
func TestTypedReferencesAreNotOwned(t *testing.T) {
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <generalization xmi:type="uml:Generalization" xmi:id="_g">
        <general xmi:type="uml:Class" href="lib.xmi#_base"/>
      </generalization>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="p">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_q" name="q">
        <type xmi:type="uml:Class" xmi:idref="_a"/>
      </ownedAttribute>
    </packagedElement>
  </uml:Model>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	g := m.Lookup("_g")
	if len(g.Children) != 0 {
		t.Errorf("generalization owns %d children, want a reference", len(g.Children))
	}
	if base := m.Ref(g, "general"); base == nil || !base.IsProxy() || base.Type != "Class" {
		t.Errorf("general = %+v", base)
	}
	p := m.Lookup("_p")
	if typ := m.Ref(p, "type"); typ == nil || !typ.IsProxy() || typ.Name != "Real" || typ.Type != "PrimitiveType" {
		t.Errorf("typed href = %+v", typ)
	}
	if len(p.Children) != 0 {
		t.Errorf("property owns %d children, want a reference", len(p.Children))
	}
	if typ := m.Ref(m.Lookup("_q"), "type"); typ == nil || typ.ID != "_a" {
		t.Errorf("typed idref = %+v", typ)
	}
}
