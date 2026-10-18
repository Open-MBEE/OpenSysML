package migrate_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A connector carrying nothing of its own — no name, type, comment or stereotype —
// whose every item flow is writable is written as its flows alone: each flow is the
// connection between the two ports, named after its item flow or, when the item flow
// is anonymous and a diagram shows the connector, after the connector's ends. The
// view exposes and routes the flows. A connector with a name of its own, or with an
// item flow that cannot be written, keeps its connection, its writable flows named.
func TestGoldenFlowOnlyConnectors(t *testing.T) {
	r := migrateStream(t, "flow_only")
	checkGolden(t, "testdata/xmi/flow_only.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/flow_only.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "flow_only.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	notation := string(r.Notation)
	for _, line := range []string{
		"flow 'supply.pwr out to load.pwr in' of Pwr from supply.'pwr out'.pwr to load.'pwr in'.pwr;",
		"flow cmd of Cmd from ctrl.bus.cmd to unit.bus.cmd;",
		"flow tlm of Tlm from unit.bus.tlm to ctrl.bus.tlm;",
		"connection 'spare feed' connect supply.'pwr out' to spare.'pwr in';",
		"flow 'spare pwr' of Pwr from supply.'pwr out'.pwr to spare.'pwr in'.pwr;",
		"connection 'supply.pwr out to aux.pwr in' connect supply.'pwr out' to aux.'pwr in';",
		"expose 'supply.pwr out to load.pwr in';",
		"expose cmd;",
		"expose tlm;",
		"expose 'spare feed';",
		"expose 'supply.pwr out to aux.pwr in';",
		"metadata DiagramLayout::Route about 'supply.pwr out to load.pwr in' { points = (120, 140, 190, 140, 190, 50, 260, 50); }",
		"metadata DiagramLayout::Route about cmd { points = (470, 80, 470, 220); }",
		"metadata DiagramLayout::Route about tlm { points = (470, 220, 470, 80); }",
		"metadata DiagramLayout::Route about 'spare feed' { points = (120, 150, 260, 150); }",
		"metadata MigrationMetadata::SynthesizedName about 'supply.pwr out to load.pwr in', 'supply.pwr out to aux.pwr in';",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, absent := range []string{
		"connect supply.'pwr out' to load.'pwr in'",
		"connect ctrl.bus to unit.bus",
		"expose 'spare pwr'",
	} {
		if strings.Contains(notation, absent) {
			t.Errorf("notation writes %q:\n%s", absent, notation)
		}
	}
	if anonymous := regexp.MustCompile(`(?m)^\s*flow of `); anonymous.MatchString(notation) {
		t.Errorf("notation writes an anonymous flow:\n%s", notation)
	}
	for id, want := range map[string]struct {
		verdict migrate.Verdict
		note    string
	}{
		"_c_pwr":   {migrate.Mapped, "written as its flow 'supply.pwr out to load.pwr in' alone"},
		"_c_bus":   {migrate.Mapped, "written as its flows cmd, tlm alone"},
		"_c_spare": {migrate.Mapped, ""},
		"_c_aux":   {migrate.Mapped, ""},
		"_if_pwr":  {migrate.Mapped, ""},
		"_if_cmd":  {migrate.Mapped, ""},
		"_if_tlm":  {migrate.Mapped, ""},
		"_if_aux":  {migrate.Unmapped, "no flow property typed by Cmd on both ends of the realizing connector"},
	} {
		es := entriesFor(r, id)
		if len(es) != 1 || es[0].Verdict != want.verdict || !strings.Contains(es[0].Note, want.note) {
			t.Errorf("entries for %s = %+v, want one %v entry noting %q", id, es, want.verdict, want.note)
		}
	}
	// The unwritten item flow 'aux' is named like the part aux, but takes no
	// name, so neither its entry nor its comment says it was written apart.
	if strings.Contains(notation, "aux 2") || strings.Contains(entriesFor(r, "_if_aux")[0].Note, "written as") {
		t.Errorf("an unwritten flow is numbered:\n%s", notation)
	}
}

// A strict migration names no OpenSysML library, so a flow's made-up name is
// listed in the comment that stands in for the SynthesizedName marker.
func TestStrictFlowOnlyConnectorsCommentTheirMadeUpNames(t *testing.T) {
	r, err := migrate.MigrateOptions("flow_only.xmi", streamArchive(t, "flow_only"), migrate.Options{Strict: true})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	wantLine(t, r.Notation, "flow 'supply.pwr out to load.pwr in' of Pwr from supply.'pwr out'.pwr to load.'pwr in'.pwr;")
	wantLine(t, r.Notation, "// names the migration made up: 'supply.pwr out to load.pwr in', 'supply.pwr out to aux.pwr in'")
	if strings.Contains(string(r.Notation), "MigrationMetadata") {
		t.Errorf("a strict migration names MigrationMetadata:\n%s", r.Notation)
	}
	for _, d := range errorsMode(t, "flow_only.sysml", r.Notation, diag.ConformanceStrict) {
		t.Errorf("%v", d)
	}
}

// flowOnlyModel is a system whose one connector realizes the item flows given,
// carrying what extra says of its own.
func flowOnlyModel(t *testing.T, extra, flows, members string) *migrate.Result {
	t.Helper()
	return migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_pwr" name="Pwr"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_heat" name="Heat"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_ib" name="iPower">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_fp_pwr" name="pwr" type="_pwr"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_fp_heat" name="heat" type="_heat"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_supply" name="Supply">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_out" name="pOut" type="_ib" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_load" name="Load">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_in" name="pIn" type="_ib" aggregation="composite" isConjugated="true"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_sys" name="Sys">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_supply" name="supply" type="_supply" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_load" name="load" type="_load" aggregation="composite"/>`+members+`
      <ownedConnector xmi:type="uml:Connector" xmi:id="_c">`+extra+`
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_pt_out" partWithPort="_p_supply"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_pt_in" partWithPort="_p_load"/>
      </ownedConnector>
    </packagedElement>`+flows, `
  <sysml:Block xmi:id="_s1" base_Class="_pwr"/>
  <sysml:Block xmi:id="_s2" base_Class="_heat"/>
  <sysml:InterfaceBlock xmi:id="_s3" base_Class="_ib"/>
  <sysml:FlowProperty xmi:id="_s4" base_Property="_fp_pwr" direction="out"/>
  <sysml:FlowProperty xmi:id="_s5" base_Property="_fp_heat" direction="out"/>
  <sysml:Block xmi:id="_s6" base_Class="_supply"/>
  <sysml:Block xmi:id="_s7" base_Class="_load"/>
  <sysml:Block xmi:id="_s8" base_Class="_sys"/>
  <sysml:ProxyPort xmi:id="_s9" base_Port="_pt_out"/>
  <sysml:ProxyPort xmi:id="_s10" base_Port="_pt_in"/>
  <sysml:ItemFlow xmi:id="_s11" base_InformationFlow="_if"/>`)
}

const powerFlow = `
    <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if" name="power" informationSource="_pt_out" informationTarget="_pt_in" conveyed="_pwr" realizingConnector="_c"/>`

// A connector with a comment of its own carries something the flow does not,
// so it keeps its connection; its flow is still named after the item flow.
func TestCommentedConnectorKeepsItsConnectionBesideItsNamedFlow(t *testing.T) {
	r := flowOnlyModel(t, `
        <ownedComment xmi:type="uml:Comment" xmi:id="_cm" body="main feed" annotatedElement="_c"/>`, powerFlow, "")
	wantLine(t, r.Notation, "connect supply.pOut to load.pIn {\n    doc /* main feed */\n}")
	wantLine(t, r.Notation, "flow power of Pwr from supply.pOut.pwr to load.pIn.pwr;")
	if es := entriesFor(r, "_c"); len(es) != 1 || es[0].Verdict != migrate.Mapped || strings.Contains(es[0].Note, "alone") {
		t.Errorf("entries for _c = %+v, want one mapped connection", es)
	}
	for _, d := range errors(t, "commented.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// An item flow named like a member of the connector's owner is written apart,
// and the flow's entry says so.
func TestFlowNamedLikeASiblingIsWrittenApart(t *testing.T) {
	r := flowOnlyModel(t, "", powerFlow, `
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_power" name="power" type="_pwr"/>`)
	wantLine(t, r.Notation, "flow 'power 2' of Pwr from supply.pOut.pwr to load.pIn.pwr;")
	if strings.Contains(string(r.Notation), "connect ") {
		t.Errorf("the connector is written beside its flow:\n%s", r.Notation)
	}
	es := entriesFor(r, "_if")
	if len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "written as power 2 since a member of Sys is also named power") {
		t.Errorf("entries for _if = %+v, want one approximation naming the clash", es)
	}
	if es := entriesFor(r, "_c"); len(es) != 1 || es[0].Target != "Model::Sys::'power 2'" {
		t.Errorf("entries for _c = %+v, want the flow as target", es)
	}
	for _, d := range errors(t, "clash.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// An item flow conveying two classifiers over a connector written as its flows
// alone is two flows, the second named apart from the first.
func TestFlowConveyingTwoClassifiersIsTwoNamedFlows(t *testing.T) {
	r := flowOnlyModel(t, "", `
    <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if" name="feed" informationSource="_pt_out" informationTarget="_pt_in" conveyed="_pwr _heat" realizingConnector="_c"/>`, "")
	wantLine(t, r.Notation, "flow feed of Pwr from supply.pOut.pwr to load.pIn.pwr;")
	wantLine(t, r.Notation, "flow 'feed 2' of Heat from supply.pOut.heat to load.pIn.heat;")
	if strings.Contains(string(r.Notation), "connect ") {
		t.Errorf("the connector is written beside its flows:\n%s", r.Notation)
	}
	if es := entriesFor(r, "_c"); len(es) != 1 || !strings.Contains(es[0].Note, "written as its flows feed, 'feed 2' alone") {
		t.Errorf("entries for _c = %+v, want one entry naming both flows", es)
	}
	for _, d := range errors(t, "two.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// An item flow conveying a classifier the migration does not write — a UML
// node has no v2 form — cannot stand for the connection, so the connector keeps it.
func TestFlowOfUnwrittenClassifierKeepsTheConnection(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Node" xmi:id="_ext" name="Ext"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_ib" name="iExt">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_fp" name="ext" type="_ext"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_supply" name="Supply">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_out" name="pOut" type="_ib" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_load" name="Load">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_in" name="pIn" type="_ib" aggregation="composite" isConjugated="true"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_sys" name="Sys">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_supply" name="supply" type="_supply" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_load" name="load" type="_load" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_c">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_pt_out" partWithPort="_p_supply"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_pt_in" partWithPort="_p_load"/>
      </ownedConnector>
    </packagedElement>
    <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if" name="ext" informationSource="_pt_out" informationTarget="_pt_in" conveyed="_ext" realizingConnector="_c"/>`, `
  <sysml:InterfaceBlock xmi:id="_s3" base_Class="_ib"/>
  <sysml:FlowProperty xmi:id="_s4" base_Property="_fp" direction="out"/>
  <sysml:Block xmi:id="_s6" base_Class="_supply"/>
  <sysml:Block xmi:id="_s7" base_Class="_load"/>
  <sysml:Block xmi:id="_s8" base_Class="_sys"/>
  <sysml:ProxyPort xmi:id="_s9" base_Port="_pt_out"/>
  <sysml:ProxyPort xmi:id="_s10" base_Port="_pt_in"/>
  <sysml:ItemFlow xmi:id="_s11" base_InformationFlow="_if"/>`)
	wantLine(t, r.Notation, "connect supply.pOut to load.pIn;")
	if es := entriesFor(r, "_c"); len(es) != 1 || strings.Contains(es[0].Note, "alone") {
		t.Errorf("entries for _c = %+v, want the connection kept", es)
	}
}
