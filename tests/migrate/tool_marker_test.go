package migrate_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// toolProfileNamespaces are the namespaces of every profile the migrator takes for the modeling tool's own.
var toolProfileNamespaces = map[string]string{
	"simulation":       "http://www.magicdraw.com/schemas/SimulationProfile.xmi",
	"dsl":              "http://www.magicdraw.com/schemas/DSL_Customization.xmi",
	"ui":               "http://www.magicdraw.com/schemas/UI_Prototyping_Profile.xmi",
	"md customization": "http://www.magicdraw.com/spec/Customization/180/SysML",
	"nomagic host":     "https://nomagic.com/schemas/DSL_Customization.xmi",
}

// toolMark applies stereotype Marker of the tool profile at ns to the element id, extending metaclass.
func toolMark(ns, metaclass, id string) string {
	return fmt.Sprintf(`<Tool:Marker xmlns:Tool=%q xmi:id="_a_%s_%s" base_%s=%q/>`, ns, id, metaclass, metaclass, id)
}

// toolMarkedDocument is a model whose root is marked by the tool profile at ns, as are a
// package, a block and the plain owners of a block, an activity and a package at every depth.
func toolMarkedDocument(ns string) []byte {
	const members = `
    <packagedElement xmi:type="uml:Package" xmi:id="_pkg" name="Marked package">
      <packagedElement xmi:type="uml:Package" xmi:id="_deep" name="Deep package">
        <packagedElement xmi:type="uml:Class" xmi:id="_deep_block" name="DeepBlock">
          <ownedAttribute xmi:type="uml:Property" xmi:id="_deep_mass" name="mass"/>
          <ownedAttribute xmi:type="uml:Property" xmi:id="_deep_derived" name="derived"/>
        </packagedElement>
        <packagedElement xmi:type="uml:Class" xmi:id="_plain" name="PlainClass">
          <ownedAttribute xmi:type="uml:Property" xmi:id="_plain_size" name="size"/>
        </packagedElement>
        <packagedElement xmi:type="uml:Component" xmi:id="_owner" name="OwnerComponent">
          <nestedClassifier xmi:type="uml:Class" xmi:id="_mid" name="MidClass">
            <nestedClassifier xmi:type="uml:Class" xmi:id="_inner" name="InnerBlock"/>
          </nestedClassifier>
        </packagedElement>
        <packagedElement xmi:type="uml:Class" xmi:id="_runner" name="Runner">
          <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run"/>
        </packagedElement>
        <packagedElement xmi:type="uml:Interface" xmi:id="_iface" name="Service">
          <nestedClassifier xmi:type="uml:Class" xmi:id="_iface_block" name="ServiceBlock"/>
        </packagedElement>
        <packagedElement xmi:type="uml:DataType" xmi:id="_dt" name="Reading"/>
        <packagedElement xmi:type="uml:Activity" xmi:id="_act" name="Operate"/>
        <packagedElement xmi:type="uml:Class" xmi:id="_tool" name="ToolOnly">
          <ownedAttribute xmi:type="uml:Property" xmi:id="_tool_p" name="setting"/>
        </packagedElement>
        <packagedElement xmi:type="uml:Component" xmi:id="_tool_c" name="ToolOnlyComponent"/>
      </packagedElement>
    </packagedElement>`
	apps := `<sysml:Block xmi:id="_a_b1" base_Class="_deep_block"/><sysml:Block xmi:id="_a_b2" base_Class="_inner"/>
  <sysml:Block xmi:id="_a_b3" base_Class="_iface_block"/>
  <sysml:ValueType xmi:id="_a_vt" base_DataType="_dt"/>`
	for _, mark := range [][2]string{
		{"Element", "_m"}, {"Package", "_m"}, {"Package", "_pkg"}, {"Element", "_deep"}, {"Package", "_deep"},
		{"Class", "_deep_block"}, {"Element", "_plain"}, {"NamedElement", "_plain_size"},
		{"Component", "_owner"}, {"Class", "_mid"}, {"Class", "_inner"}, {"Class", "_runner"},
		{"Interface", "_iface"}, {"DataType", "_dt"}, {"Activity", "_act"}, {"Element", "_deep_mass"}, {"Property", "_deep_derived"},
		{"Class", "_tool"}, {"Property", "_tool_p"}, {"Component", "_tool_c"},
	} {
		apps += "\n  " + toolMark(ns, mark[0], mark[1])
	}
	return document(members, apps)
}

// A tool profile's stereotype on an element that is or holds model content — the root, a package,
// a block, an owner of one at any depth, a behavior or its owner — annotates it: the content is written.
func TestToolMarkerKeepsModelContentOfEveryProfile(t *testing.T) {
	for name, ns := range toolProfileNamespaces {
		t.Run(name, func(t *testing.T) {
			r, err := migrate.MigrateOptions("t.xmi", toolMarkedDocument(ns), migrate.Options{})
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range []string{
				"package 'Marked package' {", "package 'Deep package' {", "part def DeepBlock {", "attribute mass {",
				"part def InnerBlock", "part def ServiceBlock", "attribute def Reading", "action def Operate", "action def Run",
				"attribute size {",
			} {
				wantLine(t, r.Notation, line)
			}
			for _, id := range []string{
				"_m", "_pkg", "_deep", "_deep_block", "_deep_mass", "_plain", "_plain_size", "_owner", "_mid", "_inner",
				"_runner", "_run", "_iface", "_iface_block", "_dt", "_act",
			} {
				for _, e := range entriesFor(r, id) {
					if e.Verdict == migrate.Skipped || e.Verdict == migrate.Unmapped {
						t.Errorf("%s is %s: %s", id, e.Verdict, e.Note)
					}
				}
				if len(entriesFor(r, id)) == 0 {
					t.Errorf("%s is not reported", id)
				}
			}
			if nothing := r.Report.NothingWritten(); nothing != "" {
				t.Errorf("NothingWritten = %q", nothing)
			}
		})
	}
}

// An element holding nothing of the model whose kind a tool profile's stereotype
// defines stays the tool's own, and is skipped with the profile's purpose.
func TestToolMarkerSkipsToolOnlyContentOfEveryProfile(t *testing.T) {
	purposes := map[string]string{
		"simulation":   "simulation tool's configuration",
		"dsl":          "specification-dialog customization",
		"ui":           "UI prototyping mockup",
		"nomagic host": "specification-dialog customization",
	}
	for name, purpose := range purposes {
		t.Run(name, func(t *testing.T) {
			r, err := migrate.MigrateOptions("t.xmi", toolMarkedDocument(toolProfileNamespaces[name]), migrate.Options{})
			if err != nil {
				t.Fatal(err)
			}
			wantNoLine(t, r.Notation, "ToolOnly")
			ids := []string{"_tool", "_tool_c"}
			if name != "simulation" {
				wantNoLine(t, r.Notation, "derived")
				ids = append(ids, "_deep_derived")
			}
			for _, id := range ids {
				es := entriesFor(r, id)
				if len(es) != 1 || es[0].Verdict != migrate.Skipped || !strings.Contains(es[0].Note, purpose) {
					t.Errorf("%s entries = %+v", id, es)
				}
			}
		})
	}
}

// A report none of whose elements is mapped or approximated says so ahead of
// its counts; one that wrote an element, or read none, does not.
func TestReportNamesAMigrationThatWroteNothing(t *testing.T) {
	empty := &migrate.Report{Entries: []migrate.Entry{
		{ID: "_m", Verdict: migrate.Skipped}, {ID: "_a", Verdict: migrate.Unmapped}, {ID: "_b", Verdict: migrate.Unmapped},
	}}
	const want = "nothing of the model was written: none of the 3 element(s) reported is mapped or approximated (2 unmapped, 1 skipped), and the report gives the reason for each"
	if got := empty.NothingWritten(); got != want {
		t.Errorf("NothingWritten = %q", got)
	}
	if !strings.HasPrefix(empty.Summary(), want+"; migrated 2 element(s): 0 mapped, 0 approximated, 2 unmapped") {
		t.Errorf("Summary = %q", empty.Summary())
	}
	for _, verdict := range []migrate.Verdict{migrate.Mapped, migrate.Approximated} {
		written := &migrate.Report{Entries: append([]migrate.Entry{{ID: "_w", Verdict: verdict}}, empty.Entries...)}
		if got := written.NothingWritten(); got != "" || !strings.HasPrefix(written.Summary(), "migrated ") {
			t.Errorf("%s: NothingWritten = %q, Summary = %q", verdict, got, written.Summary())
		}
	}
	if got := (&migrate.Report{}).NothingWritten(); got != "" {
		t.Errorf("NothingWritten of an empty report = %q", got)
	}
}

// The metaclass a stereotype extends is read alike from a base_* child element:
// the root's marker annotates, a plain class's own-kind marker makes it the tool's.
func TestToolMarkerBaseWrittenAsChildElement(t *testing.T) {
	const ns = `xmlns:Tool="http://www.magicdraw.com/schemas/DSL_Customization.xmi"`
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_noted" name="Noted"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_tool" name="ToolOnly"/>`,
		`<sysml:Block xmi:id="_a_pump" base_Class="_pump"/>
  <Tool:NumberOwner `+ns+` xmi:id="_a_m"><base_Element xmi:idref="_m"/></Tool:NumberOwner>
  <Tool:NumberOwner `+ns+` xmi:id="_a_n"><base_Element xmi:idref="_noted"/></Tool:NumberOwner>
  <Tool:Customization `+ns+` xmi:id="_a_t"><base_Class xmi:idref="_tool"/></Tool:Customization>`)
	wantLine(t, r.Notation, "part def Pump")
	wantLine(t, r.Notation, "occurrence def Noted {")
	wantNoLine(t, r.Notation, "ToolOnly")
	if es := entriesFor(r, "_tool"); len(es) != 1 || es[0].Verdict != migrate.Skipped {
		t.Errorf("_tool entries = %+v", es)
	}
}
