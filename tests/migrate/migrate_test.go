package migrate_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

var update = flag.Bool("update", false, "rewrite the golden migration outputs")

const fixture = "testdata/xmi/vehicle.xmi"

func migrateFixture(t *testing.T) *migrate.Result {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.Migrate("vehicle.xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return r
}

func checkGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the migration output (run with -update after reviewing):\n%s", path, got)
	}
}

func TestGoldenNotation(t *testing.T) {
	r := migrateFixture(t)
	checkGolden(t, "testdata/xmi/vehicle.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/vehicle.golden.report.txt", report.Bytes())
}

// errors returns the error diagnostics the analyser reports for notation.
func errors(t *testing.T, name string, notation []byte) []diag.Diagnostic {
	t.Helper()
	ws := model.NewWorkspace()
	ws.Open(name, notation, 1)
	var errs []diag.Diagnostic
	for _, d := range ws.Diagnostics(name) {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d)
		}
	}
	return errs
}

func TestMigratedNotationAnalysesClean(t *testing.T) {
	r := migrateFixture(t)
	for _, d := range errors(t, "vehicle.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// The migrated notation must survive the RDF mapping: notation -> Turtle ->
// notation -> Turtle yields the same structural graph, and the written
// notation analyses. Recorded source text is stripped between the hops so
// the structural predicates carry the round trip.
func TestMigratedNotationRoundTripsThroughTurtle(t *testing.T) {
	r := migrateFixture(t)
	hop1, err := convert.Convert("vehicle.sysml", r.Notation, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("notation -> Turtle: %v", err)
	}
	g1, err := rdf.ParseTurtle(hop1)
	if err != nil {
		t.Fatal(err)
	}
	sourceText := func(tr rdf.Triple) bool {
		return tr.Predicate == rdf.OpenSysMLTerm("sourceText") || tr.Predicate == rdf.OpenSysMLTerm("sourceTail")
	}
	structural := rdf.NewGraph()
	for _, tr := range g1.Triples() {
		if !sourceText(tr) {
			structural.AddTriple(tr)
		}
	}
	if structural.Len() == g1.Len() {
		t.Fatal("no sourceText was recorded, so stripping it proves nothing")
	}
	back, err := convert.Convert("vehicle.ttl", rdf.WriteTurtle(structural), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("Turtle -> notation: %v", err)
	}
	for _, d := range errors(t, "back.sysml", back) {
		t.Errorf("written-back notation: %v", d)
	}
	hop2, err := convert.Convert("back.sysml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("written-back notation -> Turtle: %v", err)
	}
	g2, err := rdf.ParseTurtle(hop2)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range structural.Triples() {
		if !g2.Has(tr) {
			t.Errorf("round trip lost %s %s %s", tr.Subject, tr.Predicate, tr.Object)
		}
	}
	for _, tr := range g2.Triples() {
		if !sourceText(tr) && !structural.Has(tr) {
			t.Errorf("round trip added %s %s %s", tr.Subject, tr.Predicate, tr.Object)
		}
	}
}

func TestNotationCoversTheFixture(t *testing.T) {
	r := migrateFixture(t)
	s := string(r.Notation)
	for _, want := range []string{
		"package 'Vehicle Design' {",
		"attribute def Mass :> ScalarValues::Real {",
		"enum def Color {",
		"port def FuelInterface {",
		"in item fuel : Fuel;",
		"part def Vehicle :> System {",
		"attribute mass : 'Vehicle Design'::'Value Types'::Mass default = 1200.0;",
		"part engine : Engine[1..2];",
		"part wheels : Wheel[4..*];",
		"ref part driver : Driver[0..1];",
		"port fuelIn : ~'Vehicle Design'::Interfaces::FuelInterface;",
		"binding 'fuel line' bind fuelIn = engine.fuelPort;",
		"flow fuelIn.fuel to engine.fuelPort.fuel;",
		"bind mass = massLimit.m;",
		"bind speedOut = engine.piston.p;",
		"satisfy requirement : Requirements::'Mass Requirement';",
		"satisfy requirement : Requirements::'Engine Mass Requirement' by engine;",
		"part engine : Motor :>> engine;",
		"connection def Drives {",
		"constraint def MassLimit {",
		"m < limit",
		"individual part def myCar :> Vehicle {",
		"attribute :>> mass = 1350.5;",
		"requirement def <R1> 'Mass Requirement' {",
		"doc /* The vehicle shall have a mass of less than 1500 kg. */",
		"requirement def <'R1.1'> 'Chassis Mass' {",
		":> RequirementDerivation::Derivation {",
		"end #RequirementDerivation::derive derivedRequirement : 'Engine Mass Requirement';",
		"verify requirement : 'Mass Requirement';",
		"allocate 'Vehicle Design'::Motor to 'Vehicle Design'::Engine;",
		"state def 'Vehicle States' {",
		"abstract action def start {",
		"action def Drive;",
		"/* not migrated: «Unit» InstanceSpecification 'kilogram'",
		"applied stereotype «Critical»",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("notation lacks %q", want)
		}
	}
}

func TestReportAccountsForEveryElement(t *testing.T) {
	r := migrateFixture(t)
	byID := map[string]migrate.Entry{}
	for _, e := range r.Report.Entries {
		if _, dup := byID[e.ID]; dup {
			t.Errorf("element %s is reported twice", e.ID)
		}
		byID[e.ID] = e
	}
	want := map[string]migrate.Verdict{
		"_blk_vehicle":      migrate.Approximated,
		"_prop_mass":        migrate.Mapped,
		"_dep_satisfy":      migrate.Mapped,
		"_dep_derive":       migrate.Mapped,
		"_dep_verify":       migrate.Mapped,
		"_conn_bind":        migrate.Mapped,
		"_prop_charger":     migrate.Approximated,
		"_sig_start":        migrate.Mapped,
		"_actor_driver":     migrate.Approximated,
		"_dep_trace":        migrate.Approximated,
		"_port_speedOut":    migrate.Approximated,
		"_sm_vehicle":       migrate.Mapped,
		"_act_drive":        migrate.Mapped,
		"_op_start":         migrate.Mapped,
		"_unit_kg":          migrate.Unmapped,
		"_dep_refine":       migrate.Mapped,
		"_dep_verify_block": migrate.Unmapped,
		"_lib_sysml":        migrate.Skipped,
		"_diag_bdd":         migrate.Approximated,
		"_pa_sysml":         migrate.Skipped,
	}
	for id, v := range want {
		e, ok := byID[id]
		if !ok {
			t.Errorf("%s is missing from the report", id)
			continue
		}
		if e.Verdict != v {
			t.Errorf("%s: verdict %s, want %s (%s)", id, e.Verdict, v, e.Note)
		}
		if v != migrate.Mapped && e.Note == "" {
			t.Errorf("%s: a %s verdict needs a note", id, v)
		}
	}
	if r.Report.Exporter != "Example UML Tool" {
		t.Errorf("exporter %q", r.Report.Exporter)
	}
	var js bytes.Buffer
	if err := r.Report.WriteJSON(&js); err != nil {
		t.Fatal(err)
	}
	var decoded migrate.Report
	if err := json.Unmarshal(js.Bytes(), &decoded); err != nil {
		t.Fatalf("report JSON: %v", err)
	}
	if len(decoded.Entries) != len(r.Report.Entries) {
		t.Errorf("JSON holds %d entries, want %d", len(decoded.Entries), len(r.Report.Entries))
	}
}

// Every unmapped element leaves a trace in the notation, so nothing is dropped silently.
func TestUnmappedElementsAreWrittenAsComments(t *testing.T) {
	r := migrateFixture(t)
	s := string(r.Notation)
	for _, e := range r.Report.Entries {
		if e.Verdict != migrate.Unmapped {
			continue
		}
		segs := strings.Split(e.Name, "::")
		name := segs[len(segs)-1]
		if !strings.Contains(s, "not migrated:") || !(strings.Contains(s, "'"+name+"'") || strings.Contains(s, "("+e.ID+")")) {
			t.Errorf("unmapped %s %s (%s) leaves no comment in the notation", e.Kind, e.Name, e.ID)
		}
	}
}

func TestMigratesMdzipArchive(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := map[string][]byte{
		"com.nomagic.ci.metamodel.project":      []byte("<?xml version=\"1.0\"?><project/>"),
		"com.nomagic.magicdraw.uml_model.model": data,
	}
	for _, name := range []string{"com.nomagic.ci.metamodel.project", "com.nomagic.magicdraw.uml_model.model"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entries[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipped, err := migrate.Migrate("vehicle.mdzip", buf.Bytes())
	if err != nil {
		t.Fatalf("Migrate(.mdzip): %v", err)
	}
	plain := migrateFixture(t)
	if !bytes.Equal(zipped.Notation, plain.Notation) {
		t.Error("the archive migrates differently from the document it holds")
	}
}

func TestRejectsNonXMI(t *testing.T) {
	if _, err := migrate.Migrate("x.xmi", []byte("<html/>")); err == nil {
		t.Error("expected an error for a non-XMI document")
	}
}

// constructFixtures are the XMI documents under testdata/xmi that each exercise
// one family of behavioral or profile constructs; their notation and report are golden.
var constructFixtures = []string{
	"plant_states",
	"station_points",
	"rig_interactions",
	"heater_receptions",
	"ported_calls",
	"empty_behaviors",
	"library_calls",
	"bundled_library",
	"user_library",
	"montecarlo",
	"montecarlo_case",
	"montecarlo_homonym",
	"weighted_decision",
	"tree_constraints",
	"realized_interfaces",
	"parking_usecases",
	"report_views",
	"org_profile",
	"profile_inheritance",
	"tool_profiles",
	"property_markers",
	"diagrams",
	"diagram_edges",
	"control_nodes",
	"exposed",
	"layout",
	"malformed_diagrams",
	"stub_actions",
	"tables",
	"metaclass_tables",
	"documents",
	"type_modifiers",
	"table_homonyms",
	"relation_subtypes",
}

// migrateFixtureFile migrates testdata/xmi/<name>.xmi.
func migrateFixtureFile(t *testing.T, name string) *migrate.Result {
	t.Helper()
	data, err := os.ReadFile("testdata/xmi/" + name + ".xmi")
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.Migrate(name+".xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return r
}

// The notation and report each construct fixture migrates to are pinned, and
// the notation analyses clean.
func TestGoldenConstructFixtures(t *testing.T) {
	for _, name := range constructFixtures {
		t.Run(name, func(t *testing.T) {
			r := migrateFixtureFile(t, name)
			checkGolden(t, "testdata/xmi/"+name+".golden.sysml", r.Notation)
			var report bytes.Buffer
			if err := r.Report.WriteText(&report); err != nil {
				t.Fatal(err)
			}
			checkGolden(t, "testdata/xmi/"+name+".golden.report.txt", report.Bytes())
			for _, d := range errors(t, name+".sysml", r.Notation) {
				t.Errorf("%v", d)
			}
		})
	}
}
