package sysmlv1

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

const fixture = "../../../../tests/migrate/testdata/xmi/vehicle.xmi"

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

func TestStereotypeExtensionIsRecorded(t *testing.T) {
	src := `<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="M"/>
  <sysml:Block xmi:id="_s" base_Class="_c">
    <xmi:Extension extender="MagicDraw">
      <foo xmi:type="uml:Diagram" xmi:id="_d" name="D"/>
    </xmi:Extension>
  </sysml:Block>
</xmi:XMI>`
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Stereotypes) != 1 {
		t.Fatalf("stereotypes = %+v", m.Stereotypes)
	}
	if _, ok := m.Stereotypes[0].Tags["Extension"]; ok {
		t.Fatalf("Extension tag = %+v", m.Stereotypes[0].Tags["Extension"])
	}
	if len(m.Extensions) != 1 {
		t.Fatalf("extensions = %+v", m.Extensions)
	}
	ext := m.Extensions[0]
	if ext.Extender != "MagicDraw" || ext.Owner != nil || len(ext.Elements) != 1 {
		t.Fatalf("extension = %+v", ext)
	}
	if got := ext.Elements[0]; got.ID != "_d" || got.Type != "uml:Diagram" || got.Name != "D" {
		t.Errorf("extension element = %+v", got)
	}
}

func TestExtensionElementValuesAreOwned(t *testing.T) {
	src := `<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="p"/>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r">
        <specification xmi:type="uml:Expression" xmi:id="_e" symbol="Power">
          <xmi:Extension extender="MagicDraw UML">
            <modelExtension>
              <operand xmi:type="uml:ElementValue" xmi:id="_ev" element="_p"/>
              <ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="D"/>
            </modelExtension>
          </xmi:Extension>
          <operand xmi:type="uml:LiteralInteger" xmi:id="_two" value="2"/>
        </specification>
      </ownedRule>
    </packagedElement>
  </uml:Model>
</xmi:XMI>`
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	e := m.Lookup("_e")
	operands := e.Owned("operand")
	if len(operands) != 2 || operands[0].ID != "_ev" || operands[0].Type != "ElementValue" || operands[0].Parent != e || operands[1].ID != "_two" {
		t.Fatalf("operands = %+v", operands)
	}
	if got := m.Ref(operands[0], "element"); got == nil || got.ID != "_p" {
		t.Errorf("element ref = %+v", got)
	}
	if len(m.Extensions) != 1 || len(m.Extensions[0].Elements) != 1 || m.Extensions[0].Elements[0].ID != "_d" {
		t.Errorf("extensions = %+v", m.Extensions)
	}
	if m.Lookup("_d") != nil {
		t.Error("the diagram in the extension was read as a model element")
	}
}

func TestStereotypeIgnoresXMIMetadata(t *testing.T) {
	src := `<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="M"/>
  <sysml:Block xmi:id="_s" xmi:uuid="u" base_Class="_c"/>
</xmi:XMI>`
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Stereotypes) != 1 {
		t.Fatalf("stereotypes = %+v", m.Stereotypes)
	}
	s := m.Stereotypes[0]
	if s.BaseID != "_c" {
		t.Errorf("BaseID = %q", s.BaseID)
	}
	if _, ok := s.Tags["uuid"]; ok {
		t.Errorf("uuid tag = %+v", s.Tags["uuid"])
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

// A profile whose namespace merely ends in ".xmi" (MagicDraw's SimulationProfile.xmi)
// applies stereotypes; it is not XMI metadata.
func TestProfileNamespaceEndingInXMIIsAStereotype(t *testing.T) {
	src := `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20131001" xmlns:Sim="http://www.magicdraw.com/schemas/SimulationProfile.xmi">
  <uml:Model xmi:id="m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="c" name="C"/>
  </uml:Model>
  <Sim:SimulationConfig xmi:id="s" base_Class="c" numberOfRuns="5"/>
</xmi:XMI>`
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	s := m.Lookup("c").Stereotype("SimulationConfig")
	if s == nil || s.Tag("numberOfRuns") != "5" || s.ID != "s" || !strings.HasSuffix(s.Namespace, "SimulationProfile.xmi") {
		t.Fatalf("SimulationConfig = %+v", s)
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

func TestParseSkipsMalformedNonXMIArchiveEntry(t *testing.T) {
	model := []byte(`<?xml version="1.0"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Model xmi:id="_m" name="M"/>
</xmi:XMI>`)
	m, err := Parse(archive(t, "", map[string][]byte{
		"model.xmi":    model,
		"settings.xml": []byte("<project><broken>"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if m.Lookup("_m") == nil {
		t.Fatal("model entry was not read")
	}

	if _, err := Parse([]byte("<project/>")); !errors.Is(err, errNotXMI) {
		t.Errorf("foreign root error = %v", err)
	}
	if _, err := Parse([]byte(`<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"><uml:Model>`)); err == nil || !strings.Contains(err.Error(), "parsing XMI") {
		t.Errorf("truncated XMI error = %v", err)
	}
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

// A proxy is named by its href fragment when that spells a name, dotted path
// included, and otherwise by the qualified name a tool's referenceExtension
// records; the extension also supplies the target's metaclass.
func TestReferenceExtensionsDescribeProxies(t *testing.T) {
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="p">
        <type href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML_dataType.Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_q" name="q">
        <type href="Lib.mdzip#eee_1045467100323_385364_62">
          <xmi:Extension extender="Some Tool">
            <referenceExtension referentPath="Standard Profile::datatypes::float" referentType="DataType"/>
          </xmi:Extension>
        </type>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r" name="r">
        <type href="Lib.mdzip#_18_0_2_baa02e2_1429562376320_528739_151515"/>
      </ownedAttribute>
    </packagedElement>
  </uml:Model>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	if typ := m.Ref(m.Lookup("_p"), "type"); typ == nil || typ.Name != "Real" || typ.QualifiedName != "" {
		t.Errorf("dotted fragment = %+v", typ)
	}
	q := m.Lookup("_q")
	typ := m.Ref(q, "type")
	if typ == nil || typ.Name != "float" || typ.Type != "DataType" || typ.QualifiedName != "Standard Profile::datatypes::float" {
		t.Errorf("described href = %+v", typ)
	}
	if len(q.Children) != 0 {
		t.Errorf("property owns %d children, want a reference", len(q.Children))
	}
	if typ := m.Ref(m.Lookup("_r"), "type"); typ == nil || typ.Name != "" || typ.Type != "" {
		t.Errorf("bare id href = %+v", typ)
	}
	if len(m.Extensions) != 1 || m.Extensions[0].Owner != q || len(m.Extensions[0].Elements) != 0 {
		t.Errorf("Extensions = %+v", m.Extensions)
	}
}

// profileDocument wraps a user profile and a model in one document: the
// profile defines stereotypes, the model's classes carry applications.
func profileDocument(profile, applications string) string {
	return `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:Org="http://www.magicdraw.com/schemas/Org_Profile.xmi" xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Profile" xmi:id="_prof" name="Org Profile">` + profile + `
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_c1" name="C1"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_c2" name="C2"/>
  </uml:Model>
` + applications + `
</xmi:XMI>`
}

func TestStereotypeDefinitionByNamespaceAndName(t *testing.T) {
	m, err := Parse([]byte(profileDocument(`
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_s1" name="Org Requirement">
        <generalization xmi:type="uml:Generalization" xmi:id="_g1">
          <general href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML.Requirement">
            <xmi:Extension extender="MagicDraw UML"><referenceExtension referentPath="SysML::Requirements::Requirement" referentType="Stereotype"/></xmi:Extension>
          </general>
        </generalization>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_a1" name="Rationale"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_s2" name="Plain"/>`,
		`<Org:Org_Requirement xmi:id="_ap1" base_Class="_c1" Rationale="why"/>
  <Org:Plain xmi:id="_ap2" base_Class="_c2"/>
  <Org:Unknown xmi:id="_ap3" base_Class="_c2"/>
  <sysml:Block xmi:id="_ap4" base_Class="_c2"/>`)))
	if err != nil {
		t.Fatal(err)
	}
	c1, c2 := m.Lookup("_c1"), m.Lookup("_c2")
	req := c1.Stereotype("Org Requirement")
	if req == nil || req.Definition != m.Lookup("_s1") {
		t.Fatalf("Org_Requirement definition = %+v", req)
	}
	if len(req.Generals) != 1 || !req.Generals[0].IsProxy() || req.Generals[0].Name != "Requirement" ||
		req.Generals[0].QualifiedName != "SysML::Requirements::Requirement" || req.Generals[0].Type != "Stereotype" {
		t.Errorf("Org_Requirement generals = %+v", req.Generals)
	}
	if plain := c2.Stereotype("Plain"); plain == nil || plain.Definition != m.Lookup("_s2") || len(plain.Generals) != 0 {
		t.Errorf("Plain = %+v", plain)
	}
	if unknown := c2.Stereotype("Unknown"); unknown == nil || unknown.Definition != nil {
		t.Errorf("Unknown = %+v", unknown)
	}
	if block := c2.Stereotype("Block"); block == nil || block.Definition != nil {
		t.Errorf("a standard application resolved to %+v", block.Definition)
	}
}

func TestStereotypeDefinitionByProfileURI(t *testing.T) {
	doc := `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:acme="http://acme.example/uml/Acme" xmlns:twin="http://acme.example/uml/Twin">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Profile" xmi:id="_p1" name="First" URI="http://acme.example/uml/Acme">
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_s1" name="Tag"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Profile" xmi:id="_p2" name="Second">
      <eAnnotations xmi:type="ecore:EAnnotation" xmi:id="_ann" source="http://www.eclipse.org/uml2/2.0.0/UML">
        <contents xmi:type="ecore:EPackage" xmi:id="_ep" name="Second" nsURI="http://acme.example/uml/Twin" nsPrefix="twin"/>
      </eAnnotations>
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_s2" name="Tag"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"/>
  </uml:Model>
  <acme:Tag xmi:id="_a1" base_Class="_c"/>
  <twin:Tag xmi:id="_a2" base_Class="_c"/>
</xmi:XMI>`
	m, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	c := m.Lookup("_c")
	if len(c.Stereotypes) != 2 || c.Stereotypes[0].Definition != m.Lookup("_s1") || c.Stereotypes[1].Definition != m.Lookup("_s2") {
		t.Errorf("definitions = %+v, %+v", c.Stereotypes[0].Definition, c.Stereotypes[1].Definition)
	}
}

func TestStereotypeDefinitionByHrefTable(t *testing.T) {
	// Two profiles derive the same XML namespace document name; only the
	// tool's stereotypesHREFS table can tell which one an application means.
	doc := `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:Org="http://www.magicdraw.com/schemas/Org.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Profile" xmi:id="_p1" name="Org">
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_s1" name="Tag"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Profile" xmi:id="_p2" name="org">
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_s2" name="Tag"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_d" name="D"/>
  </uml:Model>
  <Org:Tag xmi:id="_a1" base_Class="_c"/>
  <Org:Other xmi:id="_a2" base_Class="_d"/>
  <xmi:Extension extender="MagicDraw UML 2024x">
    <stereotypesHREFS>
      <stereotype name="Org:Tag" stereotypeHREF="local:/PROJECT-1?resource=com.nomagic.magicdraw.uml_umodel.model#_s2"/>
      <stereotype name="Org:Other" stereotypeHREF="local:/PROJECT-2?resource=com.nomagic.magicdraw.uml_umodel.shared_umodel#_elsewhere"/>
    </stereotypesHREFS>
  </xmi:Extension>
</xmi:XMI>`
	m, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if s := m.Lookup("_c").Stereotype("Tag"); s.Definition != m.Lookup("_s2") {
		t.Errorf("Tag definition = %+v, want the table's _s2", s.Definition)
	}
	if s := m.Lookup("_d").Stereotype("Other"); s.Definition != nil {
		t.Errorf("Other, defined in another project, resolved to %+v", s.Definition)
	}
	m2, err := Parse([]byte(strings.Replace(strings.Replace(doc, `<stereotype name="Org:Tag" stereotypeHREF="local:/PROJECT-1?resource=com.nomagic.magicdraw.uml_umodel.model#_s2"/>`, "", 1),
		`<packagedElement xmi:type="uml:Profile" xmi:id="_p2" name="org">
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_s2" name="Tag"/>
    </packagedElement>`, "", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if s := m2.Lookup("_c").Stereotype("Tag"); s.Definition != m2.Lookup("_s1") {
		t.Errorf("without the table, the single namesake = %+v", s.Definition)
	}
	if s := m.Lookup("_c").Stereotype("Tag"); s != nil {
		// Ambiguous namesakes without a table row settle nothing.
		m3, err := Parse([]byte(strings.Replace(doc, `<stereotype name="Org:Tag" stereotypeHREF="local:/PROJECT-1?resource=com.nomagic.magicdraw.uml_umodel.model#_s2"/>`, "", 1)))
		if err != nil {
			t.Fatal(err)
		}
		if s := m3.Lookup("_c").Stereotype("Tag"); s.Definition != nil {
			t.Errorf("ambiguous namesakes resolved to %+v", s.Definition)
		}
	}
}

func TestStereotypeAncestry(t *testing.T) {
	m, err := Parse([]byte(profileDocument(`
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_base" name="Base">
        <generalization xmi:type="uml:Generalization" xmi:id="_g0">
          <general href="pathmap://SysML_PROFILES/SysML.profile.uml#SysML.package_packagedElement_Blocks.stereotype_packagedElement_Block"/>
        </generalization>
      </packagedElement>
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_left" name="Left">
        <generalization xmi:type="uml:Generalization" xmi:id="_g1" general="_base"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_right" name="Right">
        <generalization xmi:type="uml:Generalization" xmi:id="_g2" general="_base"/>
        <generalization xmi:type="uml:Generalization" xmi:id="_g3" general="_missing"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_diamond" name="Diamond">
        <generalization xmi:type="uml:Generalization" xmi:id="_g4" general="_left"/>
        <generalization xmi:type="uml:Generalization" xmi:id="_g5" general="_right"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_ping" name="Ping">
        <generalization xmi:type="uml:Generalization" xmi:id="_g6" general="_pong"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_pong" name="Pong">
        <generalization xmi:type="uml:Generalization" xmi:id="_g7" general="_ping"/>
      </packagedElement>`,
		`<Org:Diamond xmi:id="_ap1" base_Class="_c1"/>
  <Org:Ping xmi:id="_ap2" base_Class="_c2"/>`)))
	if err != nil {
		t.Fatal(err)
	}
	names := func(es []*Element) string {
		var out []string
		for _, e := range es {
			out = append(out, e.Name)
		}
		return strings.Join(out, " ")
	}
	diamond := m.Lookup("_c1").Stereotype("Diamond")
	if got := names(diamond.Generals); got != "Left Right Base Block" {
		t.Errorf("diamond ancestors = %q", got)
	}
	if got := m.UnresolvedGenerals(m.Lookup("_right")); len(got) != 1 || got[0] != "_missing" {
		t.Errorf("unresolved generals = %v", got)
	}
	if block := diamond.Generals[3]; !block.IsProxy() || block.Href != "pathmap://SysML_PROFILES/SysML.profile.uml#SysML.package_packagedElement_Blocks.stereotype_packagedElement_Block" {
		t.Errorf("pathmap general = %+v", block)
	}
	ping := m.Lookup("_c2").Stereotype("Ping")
	if got := names(ping.Generals); got != "Pong Ping" {
		t.Errorf("cyclic ancestors = %q", got)
	}
	if got := names(m.Ancestors(m.Lookup("_pong"))); got != "Ping Pong" {
		t.Errorf("Pong ancestors = %q", got)
	}
}
