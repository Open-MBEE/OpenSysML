package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/convert"
	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// document wraps model members and stereotype applications in an XMI document.
func document(members, applications string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
` + members + `
  </uml:Model>
` + applications + `
</xmi:XMI>`)
}

func migrateDocument(t *testing.T, members, applications string) *migrate.Result {
	t.Helper()
	r, err := migrate.Migrate("t.xmi", document(members, applications))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func entriesFor(r *migrate.Result, id string) []migrate.Entry {
	var out []migrate.Entry
	for _, e := range r.Report.Entries {
		if e.ID == id {
			out = append(out, e)
		}
	}
	return out
}

func wantLine(t *testing.T, notation []byte, line string) {
	t.Helper()
	if !strings.Contains(string(notation), line) {
		t.Errorf("notation lacks %q:\n%s", line, notation)
	}
}

// flowModel is a system whose connector lists the flow's target end first,
// and whose two ports carry differently named flow properties for each item.
const flowModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_heat" name="Heat"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_out_if" name="Outlet">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_out_fuel" name="fuelOut" type="_fuel"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_out_heat" name="heatOut" type="_heat"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_in_if" name="Inlet">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_in_fuel" name="fuelIn" type="_fuel"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_sys" name="System">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_src" name="source" type="_out_if" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_dst" name="sink" type="_in_if" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_conn">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_dst"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_src"/>
      </ownedConnector>
    </packagedElement>
    <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if" informationSource="_src" informationTarget="_dst" conveyed="_fuel" realizingConnector="_conn"/>
    <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if2" informationSource="_src" informationTarget="_dst" conveyed="_fuel _heat" realizingConnector="_conn"/>`

const flowApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_sys"/>
  <sysml:Block xmi:id="_s2" base_Class="_fuel"/>
  <sysml:Block xmi:id="_s3" base_Class="_heat"/>
  <sysml:InterfaceBlock xmi:id="_s4" base_Class="_out_if"/>
  <sysml:InterfaceBlock xmi:id="_s5" base_Class="_in_if"/>
  <sysml:FlowProperty xmi:id="_s6" base_Property="_out_fuel" direction="out"/>
  <sysml:FlowProperty xmi:id="_s7" base_Property="_out_heat" direction="out"/>
  <sysml:FlowProperty xmi:id="_s8" base_Property="_in_fuel" direction="in"/>
  <sysml:ProxyPort xmi:id="_s9" base_Port="_src"/>
  <sysml:ProxyPort xmi:id="_s10" base_Port="_dst"/>
  <sysml:ItemFlow xmi:id="_s11" base_InformationFlow="_if"/>
  <sysml:ItemFlow xmi:id="_s12" base_InformationFlow="_if2"/>`

func TestItemFlowFollowsSourceAndTargetNotEndOrder(t *testing.T) {
	r := migrateDocument(t, flowModel, flowApplications)
	wantLine(t, r.Notation, "connect sink to source;")
	wantLine(t, r.Notation, "flow source.fuelOut to sink.fuelIn;")
	if strings.Contains(string(r.Notation), "flow sink.") {
		t.Errorf("a flow runs from the sink:\n%s", r.Notation)
	}
	if es := entriesFor(r, "_if"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("entries for _if = %+v", es)
	}
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("migrated notation has errors: %v", diags)
	}
}

func TestMultiItemFlowReportsOnce(t *testing.T) {
	r := migrateDocument(t, flowModel, flowApplications)
	if n := strings.Count(string(r.Notation), "flow source.fuelOut to sink.fuelIn;"); n != 2 {
		t.Errorf("fuel flow written %d times, want 2 (one per item flow)", n)
	}
	es := entriesFor(r, "_if2")
	if len(es) != 1 {
		t.Fatalf("entries for _if2 = %+v, want one", es)
	}
	if es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "Heat") {
		t.Errorf("entry for _if2 = %+v, want approximated over the unwritten Heat flow", es[0])
	}
}

func TestMultiEndedDependenciesWriteEveryPair(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_d" name="D"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_alloc" client="_a _b" supplier="_c _d"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_req1" name="R1"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_req2" name="R2"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_sat" client="_a" supplier="_req1 _req2"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_sat_half" client="_a" supplier="_req1 _b"/>`
	applications := `
  <sysml:Block xmi:id="_s1" base_Class="_a"/>
  <sysml:Block xmi:id="_s2" base_Class="_b"/>
  <sysml:Block xmi:id="_s3" base_Class="_c"/>
  <sysml:Block xmi:id="_s4" base_Class="_d"/>
  <sysml:Requirement xmi:id="_s5" base_Class="_req1"/>
  <sysml:Requirement xmi:id="_s6" base_Class="_req2"/>
  <sysml:Allocate xmi:id="_s7" base_Abstraction="_alloc"/>
  <sysml:Satisfy xmi:id="_s8" base_Abstraction="_sat"/>
  <sysml:Satisfy xmi:id="_s9" base_Abstraction="_sat_half"/>`
	r := migrateDocument(t, members, applications)
	for _, line := range []string{
		"allocate A to C;", "allocate A to D;", "allocate B to C;", "allocate B to D;",
		"satisfy requirement : R1;", "satisfy requirement : R2;",
	} {
		wantLine(t, r.Notation, line)
	}
	if n := strings.Count(string(r.Notation), "satisfy requirement : R1;"); n != 2 {
		t.Errorf("R1 satisfied %d times, want 2", n)
	}
	for id, want := range map[string]migrate.Verdict{"_alloc": migrate.Approximated, "_sat": migrate.Approximated, "_sat_half": migrate.Approximated} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != want {
			t.Errorf("entries for %s = %+v, want one %v", id, es, want)
		}
	}
	if es := entriesFor(r, "_sat_half"); len(es) == 1 && !strings.Contains(es[0].Note, "1 of 2") {
		t.Errorf("_sat_half note = %q, want the unwritten pair counted", es[0].Note)
	}
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("migrated notation has errors: %v", diags)
	}
}

func TestRequirementKeepsCommentWithBodyElement(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_req" name="R">
      <ownedComment xmi:type="uml:Comment" xmi:id="_cmt">
        <body>Rationale in a body element.</body>
        <annotatedElement xmi:idref="_req"/>
      </ownedComment>
    </packagedElement>`
	applications := `
  <sysml:Requirement xmi:id="_s1" base_Class="_req" Id="R-1" Text="The system shall."/>`
	r := migrateDocument(t, members, applications)
	wantLine(t, r.Notation, "doc /* The system shall. */")
	wantLine(t, r.Notation, "comment /* Rationale in a body element. */")
	if es := entriesFor(r, "_cmt"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("entries for _cmt = %+v", es)
	}
}

func TestNamedMultiPairDependencyKeepsNamesDistinct(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_d" name="uses" client="_a" supplier="_b _c"/>`,
		`<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/><sysml:Block xmi:id="_s3" base_Class="_c"/>`)
	wantLine(t, r.Notation, "dependency uses from A to B;")
	wantLine(t, r.Notation, "dependency 'uses 2' from A to C;")
	if es := entriesFor(r, "_d"); len(es) != 1 || es[0].Verdict != migrate.Approximated {
		t.Errorf("entries = %+v", es)
	}
	for _, d := range errors(t, "named.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

func TestNamedBindingConnectorKeepsItsName(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_s" name="S">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_y" name="y"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_bc" name="tie">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_x"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_y"/>
      </ownedConnector>
    </packagedElement>`,
		`<sysml:Block xmi:id="_s1" base_Class="_s"/><sysml:BindingConnector xmi:id="_s2" base_Connector="_bc"/>`)
	wantLine(t, r.Notation, "binding tie bind x = y;")
	if es := entriesFor(r, "_bc"); len(es) != 1 || es[0].Target != "S::tie" {
		t.Errorf("entries = %+v", es)
	}
	for _, d := range errors(t, "binding.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

func TestDependencyStereotypeTagsAreKept(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_a" supplier="_b"/>`,
		`<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/>
  <custom:Critical xmlns:custom="http://example.com/custom" xmi:id="_s3" base_Dependency="_d" level="high"/>
  <custom:Reviewed xmlns:custom="http://example.com/custom" xmi:id="_s4" base_Dependency="_d" by="QA"/>`)
	wantLine(t, r.Notation, "dependency A to B;")
	wantLine(t, r.Notation, "/* applied stereotype «Critical»: level = high */")
	wantLine(t, r.Notation, "/* applied stereotype «Reviewed»: by = QA */")
	es := entriesFor(r, "_d")
	if len(es) != 1 || !strings.Contains(es[0].Note, "«Critical» «Reviewed»") {
		t.Errorf("entries = %+v", es)
	}
}

func TestExternalEndKeepsLocalDependencyPairs(t *testing.T) {
	for name, members := range map[string]string{
		"external supplier": `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_a" supplier="_b">
      <supplier href="other.xmi#_ext"/>
    </packagedElement>`,
		"external client": `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_a" supplier="_b">
      <client href="other.xmi#_ext"/>
    </packagedElement>`,
	} {
		t.Run(name, func(t *testing.T) {
			r := migrateDocument(t, members, `<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/>`)
			wantLine(t, r.Notation, "dependency A to B;")
			es := entriesFor(r, "_d")
			if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "1 pair(s) reach outside the document") {
				t.Errorf("entries = %+v", es)
			}
			for _, d := range errors(t, "ext.sysml", r.Notation) {
				t.Errorf("%v", d)
			}
		})
	}
}

func TestExternalPairsAreEachCounted(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_a _b" supplier="_c">
      <supplier href="other.xmi#_ext"/>
    </packagedElement>`,
		`<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/><sysml:Block xmi:id="_s3" base_Class="_c"/>`)
	wantLine(t, r.Notation, "dependency A to C;")
	wantLine(t, r.Notation, "dependency B to C;")
	es := entriesFor(r, "_d")
	if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.HasPrefix(es[0].Note, "2 of 4 relationships written; 2 pair(s) reach outside") {
		t.Errorf("entries = %+v", es)
	}
}

func TestDanglingEndsAreEachCounted(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_a _gone" supplier="_b _c"/>`,
		`<sysml:Block xmi:id="_s1" base_Class="_a"/><sysml:Block xmi:id="_s2" base_Class="_b"/><sysml:Block xmi:id="_s3" base_Class="_c"/>`)
	wantLine(t, r.Notation, "dependency A to B;")
	wantLine(t, r.Notation, "dependency A to C;")
	es := entriesFor(r, "_d")
	if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.HasPrefix(es[0].Note, "2 of 4 relationships written; 1 client reference(s) resolve to nothing") {
		t.Errorf("entries = %+v", es)
	}
}

func TestRepeatedAnonymousDerivationsGetDistinctNames(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_r1" name="R1"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_r2" name="R2"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_r3" name="R3"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_d1" client="_r3" supplier="_r1"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_d2" client="_r3" supplier="_r2"/>`,
		`<sysml:Requirement xmi:id="_s1" base_Class="_r1"/><sysml:Requirement xmi:id="_s2" base_Class="_r2"/><sysml:Requirement xmi:id="_s3" base_Class="_r3"/>
  <sysml:DeriveReqt xmi:id="_s4" base_Abstraction="_d1"/><sysml:DeriveReqt xmi:id="_s5" base_Abstraction="_d2"/>`)
	wantLine(t, r.Notation, "connection def 'Derive R3' :> RequirementDerivation::Derivation {")
	wantLine(t, r.Notation, "connection def 'Derive R3 2' :> RequirementDerivation::Derivation {")
	for _, d := range errors(t, "derive.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	if _, err := convert.Convert("derive.sysml", r.Notation, convert.FormatSysML, convert.FormatTurtle); err != nil {
		t.Fatal(err)
	}
}

// TestShadowedReferencesAreGlobal checks a reference whose relative spelling a
// nearer declaration would capture is written from the global namespace.
func TestShadowedReferencesAreGlobal(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Package" xmi:id="_p" name="P">
      <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Package" xmi:id="_q" name="Q">
      <packagedElement xmi:type="uml:Class" xmi:id="_qa" name="A"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_qp" name="P"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_h" name="H">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x" type="_a" aggregation="composite"/>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_y" name="y" type="_b" aggregation="composite"/>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_z" name="z" type="_qa" aggregation="composite"/>
      </packagedElement>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_a"/>
  <sysml:Block xmi:id="_s2" base_Class="_b"/>
  <sysml:Block xmi:id="_s3" base_Class="_qa"/>
  <sysml:Block xmi:id="_s4" base_Class="_qp"/>
  <sysml:Block xmi:id="_s5" base_Class="_h"/>`)
	wantLine(t, r.Notation, "part x : $::A;")
	wantLine(t, r.Notation, "part y : $::P::B;")
	wantLine(t, r.Notation, "part z : A;")
	for _, d := range errors(t, "shadow.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	if _, err := convert.Convert("shadow.sysml", r.Notation, convert.FormatSysML, convert.FormatTurtle); err != nil {
		t.Fatal(err)
	}
}

// TestUnwritableFeaturePartsAreDroppedWithNotes covers three v1 shapes that
// have no v2 spelling: an anonymous property whose type is not migrated, a
// multiplicity whose bounds are strings, a non-finite real, and a default
// on a reference. Each is written validly and every loss is reported.
func TestUnwritableFeaturePartsAreDroppedWithNotes(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_v" name="Intro"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_h" name="H">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_anon" type="_v" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_grid" name="grid">
        <lowerValue xmi:type="uml:LiteralString" xmi:id="_l" value="492x21"/>
        <upperValue xmi:type="uml:LiteralString" xmi:id="_u" value="492x21"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_many" name="many">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_l2" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_u2" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_nan" name="margin">
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_d" value="NaN"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_exp" name="exposure">
        <defaultValue xmi:type="uml:LiteralString" xmi:id="_d2" value="4"/>
      </ownedAttribute>
    </packagedElement>`, `
  <sysml:View xmi:id="_s1" base_Class="_v"/>
  <sysml:Block xmi:id="_s2" base_Class="_h"/>`)
	wantLine(t, r.Notation, "ref intro;")
	wantLine(t, r.Notation, "ref grid;")
	wantLine(t, r.Notation, "ref many[0..*];")
	wantLine(t, r.Notation, "ref margin {")
	wantLine(t, r.Notation, `ref exposure default = "4";`)
	if strings.Contains(string(r.Notation), "= NaN") {
		t.Errorf("a non-finite real was written:\n%s", r.Notation)
	}
	wantLine(t, r.Notation, "/* default value not migrated: NaN")
	for id, want := range map[string]string{
		"_anon": "is named intro",
		"_grid": "multiplicity 492x21..492x21 is not a range of natural numbers",
		"_nan":  `real literal "NaN" is not a finite number`,
	} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, want) {
			t.Errorf("entries for %s = %+v, want one approximation noting %q", id, es, want)
		}
	}
	for _, d := range errors(t, "drop.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	if _, err := convert.Convert("drop.sysml", r.Notation, convert.FormatSysML, convert.FormatTurtle); err != nil {
		t.Fatal(err)
	}
}

// TestClashingSiblingNamesAreDistinguished covers two members of one classifier
// that share a name, which UML allows and v2 does not: the later one is renamed,
// references and redefinitions follow the new name, and the report says so.
func TestClashingSiblingNamesAreDistinguished(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_h" name="H">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p1" name="unnamed1"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p2" name="unnamed1"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_g" name="G">
      <generalization xmi:type="uml:Generalization" xmi:id="_gen" general="_h"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p3" name="q" redefinedProperty="_p2"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_c2" name="C"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_u" name="U">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p4" name="x" type="_c2" aggregation="composite"/>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_h"/>
  <sysml:Block xmi:id="_s2" base_Class="_g"/>
  <sysml:Block xmi:id="_s3" base_Class="_c"/>
  <sysml:Block xmi:id="_s4" base_Class="_c2"/>
  <sysml:Block xmi:id="_s5" base_Class="_u"/>`)
	wantLine(t, r.Notation, "ref unnamed1;")
	wantLine(t, r.Notation, "ref 'unnamed1 2';")
	wantLine(t, r.Notation, "ref q :>> 'unnamed1 2';")
	wantLine(t, r.Notation, "part def 'C 2'")
	wantLine(t, r.Notation, "part x : 'C 2';")
	for id, want := range map[string]string{
		"_p2": "written as unnamed1 2 since a sibling is also named unnamed1",
		"_c2": "written as C 2 since a sibling is also named C",
	} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, want) {
			t.Errorf("entries for %s = %+v, want one approximation noting %q", id, es, want)
		}
	}
	for _, d := range errors(t, "clash.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	if _, err := convert.Convert("clash.sysml", r.Notation, convert.FormatSysML, convert.FormatTurtle); err != nil {
		t.Fatal(err)
	}
}

// TestConnectionEndYieldsItsNameToAMember covers a v1 association block whose
// participant property is named like the member end it stands for: the end,
// declared in another class, takes a distinct name inside the connection def.
func TestConnectionEndYieldsItsNameToAMember(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_toB" name="toB" type="_b" association="_ab"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_toA" name="toA" type="_a" association="_ab"/>
    </packagedElement>
    <packagedElement xmi:type="uml:AssociationClass" xmi:id="_ab" name="AB" memberEnd="_toB _toA">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_pp" name="toB" type="_b"/>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_a"/>
  <sysml:Block xmi:id="_s2" base_Class="_b"/>
  <sysml:Block xmi:id="_s3" base_Class="_ab"/>
  <sysml:ParticipantProperty xmi:id="_s4" base_Property="_pp" end="_toB"/>`)
	wantLine(t, r.Notation, "end toB2 : B;")
	wantLine(t, r.Notation, "end toA : A;")
	wantLine(t, r.Notation, "ref part toB : B")
	es := entriesFor(r, "_ab")
	if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "end toB is written as toB2") {
		t.Errorf("entries for the association = %+v, want one approximation noting the renamed end", es)
	}
	for _, d := range errors(t, "end.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	if _, err := convert.Convert("end.sysml", r.Notation, convert.FormatSysML, convert.FormatTurtle); err != nil {
		t.Fatal(err)
	}
}

// TestExternalSpecializationsAreNotWritten covers a property redefining or
// subsetting a feature of another document: the clause is omitted with a note
// rather than written as a reference nothing declares.
func TestExternalSpecializationsAreNotWritten(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_h" name="H">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p1" name="a">
        <redefinedProperty href="other.xmi#_ext_a"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p2" name="b">
        <subsettedProperty href="other.xmi#_ext_b"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i" name="h1" classifier="_h">
      <slot xmi:type="uml:Slot" xmi:id="_slot">
        <definingFeature href="other.xmi#_ext_a"/>
        <value xmi:type="uml:LiteralInteger" xmi:id="_v" value="1"/>
      </slot>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_h"/>`)
	wantLine(t, r.Notation, "ref a;")
	wantLine(t, r.Notation, "ref b;")
	if strings.Contains(string(r.Notation), "_ext") || strings.Contains(string(r.Notation), "unnamed") {
		t.Errorf("notation refers to an external feature:\n%s", r.Notation)
	}
	for id, want := range map[string]string{
		"_p1": "redefinedProperty (other.xmi#_ext_a) is not written",
		"_p2": "subsettedProperty (other.xmi#_ext_b) is not written",
	} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, want) {
			t.Errorf("entries for %s = %+v, want one approximation noting %q", id, es, want)
		}
	}
	slot := entriesFor(r, "_slot")
	if len(slot) != 1 || slot[0].Verdict != migrate.Unmapped || !strings.Contains(slot[0].Note, "defining feature is not in the document") {
		t.Errorf("entries for _slot = %+v, want one unmapped entry for the external defining feature", slot)
	}
	for _, d := range errors(t, "external.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// TestBrokenNestedPathFailsTheConnector covers a NestedConnectorEnd whose
// property path names an element the document lacks, first, in the middle or
// last: the connector and the item flow it realizes are unmapped, never a
// shortened path to some other feature.
func TestBrokenNestedPathFailsTheConnector(t *testing.T) {
	for _, tc := range []struct{ name, path string }{
		{"first", `<propertyPath xmi:idref="_missing"/><propertyPath xmi:idref="_p_car"/><propertyPath xmi:idref="_p_engine"/>`},
		{"middle", `<propertyPath xmi:idref="_p_car"/><propertyPath xmi:idref="_missing"/><propertyPath xmi:idref="_p_engine"/>`},
		{"last", `<propertyPath xmi:idref="_p_car"/><propertyPath xmi:idref="_p_engine"/><propertyPath xmi:idref="_missing"/>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_piston" name="Piston">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_in" name="in" type="_fuel"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_piston" name="piston" type="_piston" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_engine" name="engine" type="_engine" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_fleet" name="Fleet">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_car" name="car" type="_car" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_out" name="out" type="_fuel"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_conn" name="feed">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_pt_out"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_pt_in"/>
      </ownedConnector>
      <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if" informationSource="_pt_out" informationTarget="_pt_in" conveyed="_fuel" realizingConnector="_conn"/>
    </packagedElement>`, `
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_fuel"/>
  <sysml:Block xmi:id="_s2" base_Class="_piston"/>
  <sysml:Block xmi:id="_s3" base_Class="_engine"/>
  <sysml:Block xmi:id="_s4" base_Class="_car"/>
  <sysml:Block xmi:id="_s5" base_Class="_fleet"/>
  <sysml:NestedConnectorEnd xmi:id="_nce" base_ConnectorEnd="_e2">`+tc.path+`</sysml:NestedConnectorEnd>
  <sysml:ItemFlow xmi:id="_st_if" base_InformationFlow="_if"/>`)
			if strings.Contains(string(r.Notation), "connect ") || strings.Contains(string(r.Notation), "flow ") {
				t.Errorf("notation writes the connector over a broken path:\n%s", r.Notation)
			}
			for id, want := range map[string]string{
				"_conn": "property path names _missing, which is not in the document",
				"_if":   "realizing connector 'feed' is not migrated: the nested connector end's property path names _missing",
			} {
				es := entriesFor(r, id)
				if len(es) != 1 || es[0].Verdict != migrate.Unmapped || !strings.Contains(es[0].Note, want) {
					t.Errorf("entries for %s = %+v, want one unmapped entry noting %q", id, es, want)
				}
			}
			for _, d := range errors(t, "nested.sysml", r.Notation) {
				t.Errorf("%v", d)
			}
		})
	}
}

// A tool may write the property path as one IDREFS attribute, the ids
// separated by whitespace and line breaks, rather than as child elements.
func TestNestedPathAsWhitespaceSeparatedAttribute(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_piston" name="Piston">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_in" name="in" type="_fuel"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_piston" name="piston" type="_piston" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_engine" name="engine" type="_engine" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_fleet" name="Fleet">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_car" name="car" type="_car" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_out" name="out" type="_fuel"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_conn" name="feed">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_pt_out"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_pt_in"/>
      </ownedConnector>
    </packagedElement>`, `
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_fuel"/>
  <sysml:Block xmi:id="_s2" base_Class="_piston"/>
  <sysml:Block xmi:id="_s3" base_Class="_engine"/>
  <sysml:Block xmi:id="_s4" base_Class="_car"/>
  <sysml:Block xmi:id="_s5" base_Class="_fleet"/>
  <sysml:NestedConnectorEnd xmi:id="_nce" base_ConnectorEnd="_e2" propertyPath="_p_car
      _p_engine _p_piston"/>`)
	wantLine(t, r.Notation, "binding feed bind 'out' = car.engine.piston.'in';")
	if es := entriesFor(r, "_conn"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("entries for _conn = %+v", es)
	}
	for _, d := range errors(t, "nested.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// multiConnectorFlow builds a flow realized by two connectors, each of whose
// second end is a nested path with the given last segment.
func multiConnectorFlow(t *testing.T, path1, path2 string) *migrate.Result {
	return migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_gas" name="Gas"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_fp" name="fuel" type="_gas"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_in" name="in" type="_fuel"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_engine" name="engine" type="_engine" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_spare" name="spare" type="_engine" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_out" name="out" type="_fuel"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_c1" name="feed1">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_pt_out"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_pt_in"/>
      </ownedConnector>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_c2" name="feed2">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e3" role="_pt_out"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e4" role="_pt_in"/>
      </ownedConnector>
      <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if" informationSource="_pt_out" informationTarget="_pt_in" conveyed="_gas" realizingConnector="_c1 _c2"/>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s0" base_Class="_gas"/>
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_fuel"/>
  <sysml:FlowProperty xmi:id="_s1f" base_Property="_fp" direction="inout"/>
  <sysml:Block xmi:id="_s3" base_Class="_engine"/>
  <sysml:Block xmi:id="_s4" base_Class="_car"/>
  <sysml:NestedConnectorEnd xmi:id="_n1" base_ConnectorEnd="_e2"><propertyPath xmi:idref="`+path1+`"/></sysml:NestedConnectorEnd>
  <sysml:NestedConnectorEnd xmi:id="_n2" base_ConnectorEnd="_e4"><propertyPath xmi:idref="`+path2+`"/></sysml:NestedConnectorEnd>
  <sysml:ItemFlow xmi:id="_st_if" base_InformationFlow="_if"/>`)
}

// TestFlowOverSeveralConnectorsIsReportedOnce covers an item flow realized by
// two connectors: one report entry whatever each connector did.
func TestFlowOverSeveralConnectorsIsReportedOnce(t *testing.T) {
	t.Run("both broken", func(t *testing.T) {
		r := multiConnectorFlow(t, "_missing1", "_missing2")
		es := entriesFor(r, "_if")
		if len(es) != 1 || es[0].Verdict != migrate.Unmapped ||
			!strings.Contains(es[0].Note, "'feed1' is not migrated") || !strings.Contains(es[0].Note, "'feed2' is not migrated") {
			t.Errorf("entries for _if = %+v, want one unmapped entry naming both connectors", es)
		}
		if n := strings.Count(string(r.Notation), "not migrated: «ItemFlow»"); n != 1 {
			t.Errorf("the flow is commented %d times, want 1:\n%s", n, r.Notation)
		}
	})
	t.Run("one broken", func(t *testing.T) {
		r := multiConnectorFlow(t, "_missing", "_p_spare")
		wantLine(t, r.Notation, "flow 'out'.fuel to spare.'in'.fuel;")
		es := entriesFor(r, "_if")
		if len(es) != 1 || es[0].Verdict != migrate.Approximated ||
			!strings.Contains(es[0].Note, "only the flow of Gas is written; realizing connector 'feed1' is not migrated") {
			t.Errorf("entries for _if = %+v, want one approximation naming feed1", es)
		}
		if strings.Contains(string(r.Notation), "not migrated: «ItemFlow»") {
			t.Errorf("a partly written flow is commented as not migrated:\n%s", r.Notation)
		}
		for _, d := range errors(t, "flows.sysml", r.Notation) {
			t.Errorf("%v", d)
		}
	})
	t.Run("both written", func(t *testing.T) {
		r := multiConnectorFlow(t, "_p_engine", "_p_spare")
		wantLine(t, r.Notation, "flow 'out'.fuel to engine.'in'.fuel;")
		wantLine(t, r.Notation, "flow 'out'.fuel to spare.'in'.fuel;")
		if es := entriesFor(r, "_if"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("entries for _if = %+v, want one mapped entry", es)
		}
	})
}

// TestMalformedRealizingConnectorSettlesItsFlow covers a flow whose realizing
// connector has three ends: the flow's entry names that failure, not an
// unmigrated owner.
func TestMalformedRealizingConnectorSettlesItsFlow(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_gas" name="Gas"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_fp" name="fuel" type="_gas"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_a" name="a" type="_fuel"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_b" name="b" type="_fuel"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_c" name="c" type="_fuel"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_c1" name="tee">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_pt_a"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_pt_b"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e3" role="_pt_c"/>
      </ownedConnector>
      <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if" informationSource="_pt_a" informationTarget="_pt_b" conveyed="_gas" realizingConnector="_c1"/>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s0" base_Class="_gas"/>
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_fuel"/>
  <sysml:FlowProperty xmi:id="_s1f" base_Property="_fp" direction="inout"/>
  <sysml:Block xmi:id="_s4" base_Class="_car"/>
  <sysml:ItemFlow xmi:id="_st_if" base_InformationFlow="_if"/>`)
	es := entriesFor(r, "_if")
	if len(es) != 1 || es[0].Verdict != migrate.Unmapped ||
		!strings.Contains(es[0].Note, "realizing connector 'tee' is not migrated: a connector with 3 ends is not migrated") ||
		strings.Contains(es[0].Note, "owned by elements that are not migrated") {
		t.Errorf("entries for _if = %+v, want one unmapped entry naming the three-ended connector", es)
	}
}

// A typed href child (<type xmi:type="uml:PrimitiveType" href=.../>) is a
// reference, so the property keeps its type and the generalization its target.
func TestTypedHrefReferencesKeepTheirTargets(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <generalization xmi:type="uml:Generalization" xmi:id="_g">
        <general xmi:type="uml:Class" href="lib.xmi#_base"/>
      </generalization>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="p">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_a"/>`)
	wantLine(t, r.Notation, "attribute p : ScalarValues::Real;")
	if es := entriesFor(r, "_p"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("property entries = %+v", es)
	}
	es := entriesFor(r, "_a")
	if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "generalization of library type") {
		t.Errorf("block entries = %+v", es)
	}
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}
}

// An opaque body written in a tool's scripting language may happen to parse
// as a v2 expression. It is copied only when every name it uses is visible
// from where it is written: parameters, inherited features, enclosing
// members; a script's library path or a bare enumeration literal is not.
func TestOpaqueExpressionsNeedVisibleNames(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Enumeration" xmi:id="_mode" name="Mode">
      <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_track" name="TRACK"/>
      <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_slew" name="SLEW"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_base" name="Timed">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_c" name="c">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_cb" name="Bound">
      <generalization xmi:type="uml:Generalization" xmi:id="_gen" general="_base"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_s" name="s">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_m_prop" name="m" type="_mode"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_d" name="d">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_dv">
          <body>s * 2.0 + c</body>
          <language>Javascript Rhino</language>
        </defaultValue>
      </ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r1" name="fits" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp1">
          <body>java.util.Collections.max(s) &lt; c</body>
          <language>Javascript Rhino</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r2" name="moving" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp2">
          <body>TRACK|SLEW</body>
          <language>Javascript Rhino</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r3" name="tracking" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp3">
          <body>m == Mode::TRACK</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>`, `
  <sysml:ValueType xmi:id="_s0" base_DataType="_mode"/>
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_base"/>
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_cb"/>`)
	wantLine(t, r.Notation, "in attribute d : ScalarValues::Real default = s * 2.0 + c;")
	wantLine(t, r.Notation, "constraint tracking { m == Mode::TRACK }")
	wantNoLine(t, r.Notation, "constraint fits")
	wantNoLine(t, r.Notation, "constraint moving")
	for id, want := range map[string]string{
		"_r1": "opaque expression names java, which nothing visible from Bound is called (language Javascript Rhino)",
		"_r2": "opaque expression names TRACK, which nothing visible from Bound is called (language Javascript Rhino)",
	} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Unmapped || es[0].Note != want {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	if diags := errors(t, "o.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}
}

// A body expression's parameters and the members it declares are names of its
// own, not of the scope the opaque body is written in; only what the body
// refers to beyond them must be visible there. A function after `->` is such a
// name, as is anything written after the expression.
func TestOpaqueBodyExpressionsBindTheirOwnNames(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_cb" name="Bound">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_s" name="s">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_sl"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_su" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_c" name="c">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r1" name="bound">
        <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_sp1">
          <body>{ in v; v > c }</body>
          <language>SysML</language>
        </defaultValue>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r2" name="scaled">
        <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_sp2">
          <body>{ in v; private attribute k = 2.0; v * k > c }</body>
          <language>SysML</language>
        </defaultValue>
      </ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r3" name="stray" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp3">
          <body>{ in v; v > limit }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r4" name="each" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp4">
          <body>s->forAll { in v; v > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r5" name="twice" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp5">
          <body>c > 0.0; attribute probe2 = c</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r6" name="bodied" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp6">
          <body>c > 0.0 { doc /* none */ }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r7" name="nested" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp7">
          <body>{ in v { attribute y = missing; } v > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r8" name="deep" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp8">
          <body>{ in v; private attribute k { attribute y = absent; } v > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r9" name="asserted" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp9">
          <body>{ in v { constraint { gone > 0 } } v > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r10" name="bounded" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp10">
          <body>{ in v; attribute y[lost]; v > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r11" name="imported" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp11">
          <body>{ in v { import Q::*; } v > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r12" name="documented">
        <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_sp12">
          <body>{ in v { doc /* the value */ } v > c }</body>
          <language>SysML</language>
        </defaultValue>
      </ownedAttribute>
    </packagedElement>`, `
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_cb"/>`)
	wantLine(t, r.Notation, "in attribute bound default = { in v; v > c };")
	wantLine(t, r.Notation, "in attribute scaled default = { in v; private attribute k = 2.0; v * k > c };")
	wantLine(t, r.Notation, "in attribute documented default = { in v { doc /* the value */ } v > c };")
	for _, n := range []string{"stray", "each", "twice", "bodied", "nested", "deep", "asserted", "bounded", "imported"} {
		wantNoLine(t, r.Notation, "constraint "+n)
	}
	for id, want := range map[string]string{
		"_r3":  "opaque expression names limit, which nothing visible from Bound is called (language SysML)",
		"_r4":  "opaque expression names forAll, which nothing visible from Bound is called (language SysML)",
		"_r5":  "opaque expression is not v2 expression syntax (language SysML)",
		"_r6":  "opaque expression is not v2 expression syntax (language SysML)",
		"_r7":  "opaque expression names missing, which nothing visible from Bound is called (language SysML)",
		"_r8":  "opaque expression names absent, which nothing visible from Bound is called (language SysML)",
		"_r9":  "opaque expression names gone, which nothing visible from Bound is called (language SysML)",
		"_r10": "opaque expression names lost, which nothing visible from Bound is called (language SysML)",
		"_r11": "opaque expression is not v2 expression syntax (language SysML)",
	} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Unmapped || es[0].Note != want {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	if diags := errors(t, "b.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}
}

// A feature chain on a body's own name is checked against the type the name
// is declared with, and a qualified name starting in a standard-library package
// against the library, which a v2 model reaches without importing it. An
// unqualified library name is still not visible, and a chain on a name declared
// without a type cannot be checked.
func TestOpaqueBodyChainsAndLibraryNamesAreChecked(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_pt" name="Pt">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_cb" name="Bound">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_c" name="c">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r1" name="pointed">
        <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_sp1">
          <body>{ in v : Pt; v.x > c }</body>
          <language>SysML</language>
        </defaultValue>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r2" name="counted">
        <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_sp2">
          <body>{ in v : ScalarValues::Integer; v > c }</body>
          <language>SysML</language>
        </defaultValue>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r3" name="weighed">
        <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_sp3">
          <body>{ in v : ISQ::mass; v.num > c }</body>
          <language>SysML</language>
        </defaultValue>
      </ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r4" name="astray" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp4">
          <body>{ in v : Pt; v.missing > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r5" name="untyped" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp5">
          <body>{ in v; v.x > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r6" name="misspelled" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp6">
          <body>{ in v : ScalarValues::Nope; v > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r7" name="unimported" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp7">
          <body>{ in v : Integer; v > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r8" name="chained" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp8">
          <body>{ in v : ScalarValues::Integer; v.missing > c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>`, `
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_cb"/>`)
	wantLine(t, r.Notation, "in attribute pointed default = { in v : Pt; v.x > c };")
	wantLine(t, r.Notation, "in attribute counted default = { in v : ScalarValues::Integer; v > c };")
	wantLine(t, r.Notation, "in attribute weighed default = { in v : ISQ::mass; v.num > c };")
	for _, n := range []string{"astray", "untyped", "misspelled", "unimported", "chained"} {
		wantNoLine(t, r.Notation, "constraint "+n)
	}
	for id, want := range map[string]string{
		"_r4": "opaque expression names v.missing, which nothing visible from Bound is called (language SysML)",
		"_r5": "opaque expression reaches v.x through v, whose type it does not declare (language SysML)",
		"_r6": "opaque expression names ScalarValues::Nope, which nothing visible from Bound is called (language SysML)",
		"_r7": "opaque expression names Integer, which nothing visible from Bound is called (language SysML)",
		"_r8": "opaque expression names v.missing, which nothing visible from Bound is called (language SysML)",
	} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Unmapped || es[0].Note != want {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	if diags := errors(t, "b.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}
}

func TestOpaqueBodiesCheckCastBoundsAndUsageEnds(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_pt" name="Pt">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_link" name="Link"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_cb" name="Bound">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_c" name="c">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_castMissing" name="castMissing" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_castMissingValue">
          <body>(as Pt[missing])</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_castPresent" name="castPresent" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_castPresentValue">
          <body>(as Pt[c])</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_connectionMissing" name="connectionMissing" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_connectionMissingValue">
          <body>{ connection link : Link connect c to missing; c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_connectionPresent" name="connectionPresent" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_connectionPresentValue">
          <body>{ connection link : Link connect c to c; c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_connectionDeclaredPresent" name="connectionDeclaredPresent" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_connectionDeclaredPresentValue">
          <body>{ connection link : Link connect src references c to dst references c { attribute x = src; } c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_connectionDeclaredMissing" name="connectionDeclaredMissing" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_connectionDeclaredMissingValue">
          <body>{ connection link : Link connect src references c to dst references c { attribute x = other; } c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_flowMissing" name="flowMissing" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_flowMissingValue">
          <body>{ flow f from c to missing; c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_flowPresent" name="flowPresent" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_flowPresentValue">
          <body>{ flow f from c to c; c }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_param" name="param" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_paramValue">
          <body>{ in v : Pt; (as Pt[v.x]) }</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>`, `
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_cb"/>`)

	for _, name := range []string{"castPresent", "connectionPresent", "connectionDeclaredPresent", "flowPresent", "param"} {
		wantLine(t, r.Notation, "constraint "+name+" { ")
	}
	for _, name := range []string{"castMissing", "connectionMissing", "connectionDeclaredMissing", "flowMissing"} {
		wantNoLine(t, r.Notation, "constraint "+name+" { ")
	}
	for _, id := range []string{"_castPresent", "_connectionPresent", "_connectionDeclaredPresent", "_flowPresent", "_param"} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "copied verbatim") {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	for id, want := range map[string]string{
		"_castMissing":               "opaque expression names missing, which nothing visible from Bound is called (language SysML)",
		"_connectionMissing":         "opaque expression names missing, which nothing visible from Bound is called (language SysML)",
		"_connectionDeclaredMissing": "opaque expression names other, which nothing visible from Bound is called (language SysML)",
		"_flowMissing":               "opaque expression names missing, which nothing visible from Bound is called (language SysML)",
	} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != migrate.Unmapped || es[0].Note != want {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
}

// isOrdered and isUnique=false are written as the ordered and nonunique
// modifiers, on properties and association ends alike.
func TestCollectionModifiersAreWritten(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_o" name="o" isOrdered="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_ol" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_ou" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_n" name="n" isUnique="false" type="_a" aggregation="composite">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_nl" value="2"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_nu" value="2"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b" isOrdered="true" isUnique="false" type="_a">
        <subsettedProperty xmi:idref="_n"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_u" name="u" isOrdered="false" isUnique="true" type="_a"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_as" name="Link" memberEnd="_e1 _e2">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e1" name="first" type="_a" isOrdered="true">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_e1l" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_e1u" value="*"/>
      </ownedEnd>
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2" name="second" type="_a"/>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_a"/>`)
	wantLine(t, r.Notation, "attribute o : ScalarValues::Real[0..*] ordered;")
	wantLine(t, r.Notation, "part n : A[2] nonunique;")
	wantLine(t, r.Notation, "ref part b : A ordered nonunique :> n;")
	wantLine(t, r.Notation, "ref part u : A;")
	wantLine(t, r.Notation, "end 'first' : A[0..*] ordered;")
	for _, id := range []string{"_o", "_n", "_b", "_u", "_e1"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}
	ttl, err := convert.Convert("t.sysml", r.Notation, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"isOrdered", "isNonunique"} {
		if !strings.Contains(string(ttl), want) {
			t.Errorf("Turtle lacks %s", want)
		}
	}
}

// Prefix keywords come out in grammar order (direction, derived, abstract, constant,
// ref); read-only is `constant` except in a value type, where it is only noted.
func TestPrefixModifiersFollowTheGrammarOrder(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ro" name="ro" isReadOnly="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_da" name="da" isDerived="true" isReadOnly="true" type="_a" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_sh" name="sh" isReadOnly="true" type="_a" aggregation="shared"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_cb" name="Limit">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_cp" name="cp" isDerived="true" isReadOnly="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:DataType" xmi:id="_vt" name="Mass">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_vp" name="vp" isReadOnly="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_if" name="I">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_fp" name="fp" isDerived="true" isReadOnly="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_a"/>
  <sysml:InterfaceBlock xmi:id="_s2" base_Class="_if"/>
  <sysml:FlowProperty xmi:id="_s3" base_Property="_fp" direction="out"/>
  <sysml:ConstraintBlock xmi:id="_s4" base_Class="_cb"/>
  <sysml:ValueType xmi:id="_s5" base_DataType="_vt"/>`)
	wantLine(t, r.Notation, "constant attribute ro : ScalarValues::Boolean;")
	wantLine(t, r.Notation, "derived constant part da : A;")
	wantLine(t, r.Notation, "constant ref part sh : A;")
	wantLine(t, r.Notation, "in derived constant attribute cp : ScalarValues::Real;")
	wantLine(t, r.Notation, "out derived constant attribute fp : ScalarValues::Real;")
	wantLine(t, r.Notation, "attribute vp : ScalarValues::Real;")
	for _, id := range []string{"_ro", "_da", "_cp", "_fp"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	for id, want := range map[string]string{"_sh": "shared aggregation", "_vp": "read-only is not written"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, want) {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}
}

// MagicDraw stores a constraint block's parameters as private ports marked with
// its customization's «ConstraintParameter»; they are the constraint's `in`
// parameters all the same, written public so the block's binding connectors
// resolve to them, and one typed by a block is an `in ref part`, since an
// attribute cannot be typed by a part def. A user profile's same-named
// stereotype is no such marker and stays on the parameter as a comment.
func TestConstraintParametersStoredAsPortsAreInParameters(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Timer">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bp" name="elapsed">
        <type href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML_dataType.Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bc" name="limit" type="_cb" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_bind">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_bp"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_p1" partWithPort="_bc"/>
      </ownedConnector>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_cb" name="MaxTime">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_p1" name="t" visibility="private" aggregation="composite">
        <type href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML_dataType.Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_p2" name="maxTime" visibility="protected" aggregation="composite">
        <type href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML_dataType.Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p3" name="slack" visibility="private">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_p4" name="inner" type="_cb2" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_p6" name="timer" type="_b" visibility="private" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p5" name="tolerance">
        <type href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML_dataType.Real"/>
      </ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_rule" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_spec">
          <body>t + slack &lt;= maxTime</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_cb2" name="Positive"/>`, `
  <sysml:Block xmi:id="_s1" base_Class="_b"/>
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_cb"/>
  <sysml:ConstraintBlock xmi:id="_s3" base_Class="_cb2"/>
  <sysml:BindingConnector xmi:id="_s4" base_Connector="_bind"/>
  <md:ConstraintParameter xmlns:md="http://www.magicdraw.com/spec/Customization/180/SysML" xmi:id="_c1" base_Port="_p1"/>
  <md:ConstraintParameter xmlns:md="http://www.magicdraw.com/spec/Customization/180/SysML" xmi:id="_c2" base_Port="_p2"/>
  <md:ConstraintParameter xmlns:md="http://www.magicdraw.com/spec/Customization/180/SysML" xmi:id="_c4" base_Port="_p4"/>
  <md:ConstraintParameter xmlns:md="http://www.magicdraw.com/spec/Customization/180/SysML" xmi:id="_c6" base_Port="_p6"/>
  <custom:ConstraintParameter xmlns:custom="http://example.com/tool/customization" xmi:id="_c5" base_Property="_p5"/>`)
	wantLine(t, r.Notation, "in attribute t : ScalarValues::Real;")
	wantLine(t, r.Notation, "in attribute maxTime : ScalarValues::Real;")
	wantLine(t, r.Notation, "in attribute slack : ScalarValues::Real;")
	wantLine(t, r.Notation, "constraint inner : Positive;")
	wantLine(t, r.Notation, "in ref part timer : Timer;")
	wantLine(t, r.Notation, "in attribute tolerance : ScalarValues::Real {")
	wantLine(t, r.Notation, "/* applied stereotype «ConstraintParameter» */")
	wantLine(t, r.Notation, "bind elapsed = limit.t;")
	wantNoLine(t, r.Notation, "port")
	if n := strings.Count(string(r.Notation), "«ConstraintParameter»"); n != 1 {
		t.Errorf("«ConstraintParameter» written %d times, want once, for the user profile's", n)
	}
	for _, id := range []string{"_p1", "_p2", "_p3", "_p6"} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "visibility is not written on a constraint parameter") {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	if es := entriesFor(r, "_p4"); len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("_p4 entries = %+v", es)
	}
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}
}

// namedRelationModel is a block, a requirement and a test case joined by one
// named dependency of each SysML relationship stereotype.
const namedRelationModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bp" name="piece" type="_b2" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b2" name="Piece"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_r" name="Req"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_r2" name="Req2"/>
    <packagedElement xmi:type="uml:Activity" xmi:id="_tc" name="Check"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_sat" name="sat" client="_bp" supplier="_r"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_ver" name="ver" client="_tc" supplier="_r"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_ref" name="ref" client="_b" supplier="_r"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_alloc" name="alloc" client="_b" supplier="_b2"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_trace" name="trace" client="_b" supplier="_r"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_copy" name="copy" client="_r2" supplier="_r"/>`

const namedRelationApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_b"/>
  <sysml:Block xmi:id="_s2" base_Class="_b2"/>
  <sysml:Requirement xmi:id="_s3" base_Class="_r" Id="R1" Text="Shall."/>
  <sysml:Requirement xmi:id="_s4" base_Class="_r2" Id="R2" Text="Shall too."/>
  <sysml:TestCase xmi:id="_s5" base_Behavior="_tc"/>
  <sysml:Satisfy xmi:id="_s6" base_Abstraction="_sat"/>
  <sysml:Verify xmi:id="_s7" base_Abstraction="_ver"/>
  <sysml:Refine xmi:id="_s8" base_Abstraction="_ref"/>
  <sysml:Allocate xmi:id="_s9" base_Abstraction="_alloc"/>
  <sysml:Trace xmi:id="_s10" base_Abstraction="_trace"/>
  <sysml:Copy xmi:id="_s11" base_Abstraction="_copy"/>`

func TestNamedRelationshipsKeepTheirNames(t *testing.T) {
	r := migrateDocument(t, namedRelationModel, namedRelationApplications)
	wantLine(t, r.Notation, "satisfy requirement sat : Req by piece;")
	wantLine(t, r.Notation, "verify requirement ver : Req;")
	wantLine(t, r.Notation, "dependency 'ref' from Thing to Req {")
	wantLine(t, r.Notation, "allocation alloc allocate Thing to Piece;")
	wantLine(t, r.Notation, "dependency trace from Thing to Req; /* «Trace» */")
	wantLine(t, r.Notation, "dependency copy from Req2 to Req; /* «Copy» */")
	for id, want := range map[string]struct {
		verdict migrate.Verdict
		target  string
	}{
		"_sat":   {migrate.Mapped, "Thing::sat"},
		"_ver":   {migrate.Mapped, "Check::ver"},
		"_ref":   {migrate.Mapped, "'ref'"},
		"_alloc": {migrate.Mapped, "alloc"},
		"_trace": {migrate.Approximated, "trace"},
		"_copy":  {migrate.Approximated, "copy"},
	} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != want.verdict || es[0].Target != want.target {
			t.Errorf("%s: entries = %+v, want %v %q", id, es, want.verdict, want.target)
		}
	}
	if _, err := convert.Convert("t.sysml", r.Notation, convert.FormatSysML, convert.FormatTurtle); err != nil {
		t.Fatalf("migrated notation does not convert: %v", err)
	}
}

func TestPlacedRelationshipNameYieldsToAMember(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Thing">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bp" name="sat" type="_b2" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b2" name="Piece"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_r" name="Req"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_sat" name="sat" client="_bp" supplier="_r"/>`, `
  <sysml:Block xmi:id="_s1" base_Class="_b"/>
  <sysml:Block xmi:id="_s2" base_Class="_b2"/>
  <sysml:Requirement xmi:id="_s3" base_Class="_r" Id="R1" Text="Shall."/>
  <sysml:Satisfy xmi:id="_s6" base_Abstraction="_sat"/>`)
	wantLine(t, r.Notation, "satisfy requirement 'sat 2' : Req by sat;")
	if es := entriesFor(r, "_sat"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "sat 2") {
		t.Errorf("entries = %+v", es)
	}
}

// An expression can only name what its v2 copy will resolve: an operation
// is not written, so a call to it is not copied; a private inherited property
// is not inherited in v2, so naming it exposes the property instead.
func TestOpaqueExpressionsNeedWrittenAccessibleNames(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_base" name="Timed">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_secret" name="secret" visibility="private">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_hidden" name="hidden" visibility="private">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_cb" name="Bound">
      <generalization xmi:type="uml:Generalization" xmi:id="_gen" general="_base"/>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_run" name="run"/>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r1" name="ran" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp1">
          <body>run() > 0</body>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r2" name="kept" constrainedElement="_cb">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp2">
          <body>secret + 1 > 0</body>
        </specification>
      </ownedRule>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_base"/>
  <sysml:Block xmi:id="_s2" base_Class="_cb"/>`)
	wantLine(t, r.Notation, "attribute secret : ScalarValues::Real;")
	wantLine(t, r.Notation, "private attribute hidden : ScalarValues::Real;")
	wantLine(t, r.Notation, "constraint kept { secret + 1 > 0 }")
	wantNoLine(t, r.Notation, "constraint ran")
	wantNote(t, r, "_r1", migrate.Unmapped, "opaque expression names run, which is the action def Bound::run, not a value an expression can read")
	wantNote(t, r, "_secret", migrate.Approximated, "private visibility is not written: an expression in Bound names it")
	wantClean(t, "access.sysml", r)
}

// Every step of a name is checked, not just its root: a chain's member must
// be a feature of the operand's type (owned or inherited, a private one
// exposed), and a qualified name's segment a member of the namespace before it.
func TestOpaqueExpressionsResolveEveryStep(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Enumeration" xmi:id="_mode" name="Mode">
      <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_track" name="TRACK"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rpm" name="rpm">`+realHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine">
      <generalization xmi:type="uml:Generalization" xmi:id="_gen" general="_motor"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_temp" name="temp">`+realHref+`</ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_wear" name="wear" visibility="private">`+realHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_e" name="engine" type="_engine" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_m_prop" name="mode" type="_mode"/>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r1" name="hot" constrainedElement="_car">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp1"><body>engine.temp > 90.0 and engine.rpm > 0.0</body></specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r2" name="worn" constrainedElement="_car">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp2"><body>engine.wear &lt; 1.0</body></specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r3" name="broken" constrainedElement="_car">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp3"><body>engine.missing > 0.0</body></specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r4" name="parked" constrainedElement="_car">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp4"><body>mode == Mode::PARK</body></specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r5" name="tracking" constrainedElement="_car">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp5"><body>mode == Mode::TRACK</body></specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r6" name="deep" constrainedElement="_car">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp6"><body>engine.temp.missing > 0.0</body></specification>
      </ownedRule>
    </packagedElement>`, `
  <sysml:ValueType xmi:id="_s0" base_DataType="_mode"/>
  <sysml:Block xmi:id="_s1" base_Class="_motor"/>
  <sysml:Block xmi:id="_s2" base_Class="_engine"/>
  <sysml:Block xmi:id="_s3" base_Class="_car"/>`)
	wantLine(t, r.Notation, "constraint hot { engine.temp > 90.0 and engine.rpm > 0.0 }")
	wantLine(t, r.Notation, "constraint worn { engine.wear < 1.0 }")
	wantLine(t, r.Notation, "constraint tracking { mode == Mode::TRACK }")
	wantLine(t, r.Notation, "attribute wear : ScalarValues::Real;")
	wantNoLine(t, r.Notation, "constraint broken")
	wantNoLine(t, r.Notation, "constraint parked")
	wantNoLine(t, r.Notation, "constraint deep")
	wantNote(t, r, "_r3", migrate.Unmapped, "opaque expression names engine.missing, which nothing visible from Car is called")
	wantNote(t, r, "_r4", migrate.Unmapped, "opaque expression names Mode::PARK, which nothing visible from Car is called")
	wantNote(t, r, "_r6", migrate.Unmapped, "opaque expression names engine.temp.missing, which nothing visible from Car is called")
	wantNote(t, r, "_wear", migrate.Approximated, "private visibility is not written: an expression in Car names it")
	wantClean(t, "steps.sysml", r)
}

// A package import is written as `public import P::*`, so an expression may name the
// members it brings in — every packaged element is written public, a private one with a note.
func TestOpaqueExpressionsMayNameImportedMembers(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Package" xmi:id="_lib" name="Modes">
      <packagedElement xmi:type="uml:Enumeration" xmi:id="_mode" name="Mode">
        <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_track" name="TRACK"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Enumeration" xmi:id="_gear" name="Gear" visibility="private">
        <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_low" name="LOW"/>
      </packagedElement>
    </packagedElement>
    <packagedElement xmi:type="uml:Package" xmi:id="_sys" name="System">
      <packageImport xmi:type="uml:PackageImport" xmi:id="_imp" importedPackage="_lib"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_m_prop" name="mode" type="_mode"/>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_g" name="gear" type="_gear"/>
        <ownedRule xmi:type="uml:Constraint" xmi:id="_r1" name="tracking" constrainedElement="_car">
          <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp1"><body>mode == Mode::TRACK</body></specification>
        </ownedRule>
        <ownedRule xmi:type="uml:Constraint" xmi:id="_r2" name="crawling" constrainedElement="_car">
          <specification xmi:type="uml:OpaqueExpression" xmi:id="_sp2"><body>gear == Gear::LOW</body></specification>
        </ownedRule>
      </packagedElement>
    </packagedElement>`, `
  <sysml:ValueType xmi:id="_s0" base_DataType="_mode"/>
  <sysml:ValueType xmi:id="_s1" base_DataType="_gear"/>
  <sysml:Block xmi:id="_s2" base_Class="_car"/>`)
	wantLine(t, r.Notation, "public import Modes::*;")
	wantLine(t, r.Notation, "constraint tracking { mode == Mode::TRACK }")
	wantLine(t, r.Notation, "constraint crawling { gear == Gear::LOW }")
	wantLine(t, r.Notation, "enum def Gear {")
	wantNote(t, r, "_gear", migrate.Approximated, "private visibility is not written: v2 lets nothing outside the package reach a private member")
	wantClean(t, "imports.sysml", r)
}
