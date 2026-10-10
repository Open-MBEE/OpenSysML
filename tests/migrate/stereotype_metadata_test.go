package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func assertStereotypeMigrationValid(t *testing.T, r *migrate.Result) {
	t.Helper()
	if ds := errors(t, "stereotype_metadata.sysml", r.Notation); len(ds) > 0 {
		t.Fatalf("migrated notation has errors: %v\n%s", ds, r.Notation)
	}
}

func TestAllocateFromActionToPartIsAnAllocationDef(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_host" name="Host">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_payload" name="payload" type="_payloadType" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_payloadType" name="PayloadType"/>
    <packagedElement xmi:type="uml:Activity" xmi:id="_run" name="Run">
      <node xmi:type="uml:InitialNode" xmi:id="_start"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_call" name="invoke" behavior="_work"/>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_finish"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_first" source="_start" target="_call"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_last" source="_call" target="_finish"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_work" name="Work">
      <node xmi:type="uml:InitialNode" xmi:id="_workStart"/>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_workFinish"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_workEdge" source="_workStart" target="_workFinish"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_allocation" client="_call" supplier="_payload"/>`
	applications := `
  <sysml:Block xmi:id="_hostBlock" base_Class="_host"/>
  <sysml:Block xmi:id="_payloadBlock" base_Class="_payloadType"/>
  <sysml:Allocate xmi:id="_allocate" base_Abstraction="_allocation"/>`
	r := migrateDocument(t, members, applications)
	assertStereotypeMigrationValid(t, r)
	for _, line := range []string{
		"allocation def 'invoke to payload' {",
		"end :>> source : Run;",
		"end :>> target : Host;",
		"allocate source.invoke to target.payload;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "dependency ") {
		t.Errorf("allocation became a dependency:\n%s", r.Notation)
	}
	es := entriesFor(r, "_allocation")
	if len(es) != 1 || es[0].Verdict != migrate.Mapped || es[0].Note != "" {
		t.Errorf("allocation entry = %+v, want Mapped without a note", es)
	}
}

// An «Allocate» whose end is a requirement, written as a usage no allocation
// end can be typed by, stays a dependency and the report says why.
func TestAllocateToARequirementStaysADependency(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_flow" name="Flow"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_allocation" client="_pump" supplier="_flow"/>`, `
  <sysml:Block xmi:id="_s1" base_Class="_pump"/>
  <sysml:Requirement xmi:id="_s2" base_Class="_flow"/>
  <sysml:Allocate xmi:id="_allocate" base_Abstraction="_allocation"/>`)
	wantLine(t, r.Notation, "requirement Flow;")
	wantNoLine(t, r.Notation, "allocation def")
	wantLine(t, r.Notation, "dependency 'Pump allocated to Flow' from Pump to Flow {")
	wantLine(t, r.Notation, `stereotype = "Allocate";`)
	wantNote(t, r, "_allocation", migrate.Approximated, "its end Flow has no enclosing written definition to type an allocation end, so a plain dependency stands for it")
	wantClean(t, "allocate-to-requirement.sysml", r)
}

func TestAllocateFromDefinitionToFeatureKeepsTheFeature(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_target" name="Target">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_y" name="y"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_source" name="Source"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_allocation" client="_source" supplier="_y"/>`
	applications := `<sysml:Block xmi:id="_block" base_Class="_target"/>
  <sysml:Allocate xmi:id="_allocate" base_Abstraction="_allocation"/>`
	r := migrateDocument(t, members, applications)
	assertStereotypeMigrationValid(t, r)
	wantLine(t, r.Notation, "allocate source to target.y;")
	if strings.Contains(string(r.Notation), "dependency ") {
		t.Errorf("allocation became a dependency:\n%s", r.Notation)
	}
}

func TestAllocateBetweenDefinitionsRedefinesSourceAndTarget(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_source" name="Source"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_target" name="Target"/>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_allocation" client="_source" supplier="_target"/>`
	applications := `
  <sysml:Block xmi:id="_sourceBlock" base_Class="_source"/>
  <sysml:Block xmi:id="_targetBlock" base_Class="_target"/>
  <sysml:Allocate xmi:id="_allocate" base_Abstraction="_allocation"/>`
	r := migrateDocument(t, members, applications)
	assertStereotypeMigrationValid(t, r)
	for _, line := range []string{
		"allocation def 'Source to Target' {",
		"end :>> source : Source;",
		"end :>> target : Target;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "allocate source") {
		t.Errorf("definition-to-definition allocation has an allocate usage:\n%s", r.Notation)
	}
	es := entriesFor(r, "_allocation")
	if len(es) != 1 || es[0].Verdict != migrate.Mapped || es[0].Note != "" {
		t.Errorf("allocation entry = %+v, want Mapped without a note", es)
	}
}

func TestAllocateInOneBodyStaysInPlace(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_owner" name="Owner">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a" name="a"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_allocation" client="_a" supplier="_b"/>`
	applications := `<sysml:Block xmi:id="_block" base_Class="_owner"/>
  <sysml:Allocate xmi:id="_allocate" base_Abstraction="_allocation"/>`
	r := migrateDocument(t, members, applications)
	assertStereotypeMigrationValid(t, r)
	wantLine(t, r.Notation, "allocate a to b;")
	if strings.Contains(string(r.Notation), "allocation def") || strings.Contains(string(r.Notation), "dependency ") {
		t.Errorf("same-body allocation changed form:\n%s", r.Notation)
	}

	pkg := migrateDocument(t, `
    <packagedElement xmi:type="uml:Package" xmi:id="_package" name="Package">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a" name="a"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b"/>
      <packagedElement xmi:type="uml:Abstraction" xmi:id="_allocation" client="_a" supplier="_b"/>
    </packagedElement>`,
		`<sysml:Allocate xmi:id="_allocate" base_Abstraction="_allocation"/>`)
	assertStereotypeMigrationValid(t, pkg)
	wantLine(t, pkg.Notation, "allocate a to b;")
	if strings.Contains(string(pkg.Notation), "allocation def") || strings.Contains(string(pkg.Notation), "dependency ") {
		t.Errorf("package-level allocation changed form:\n%s", pkg.Notation)
	}
}

func TestAllocateFromPackageFeatureStaysADependency(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Package" xmi:id="_package" name="Package">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_packageFeature" name="packageFeature"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_owner" name="Owner">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_part" name="part"/>
      </packagedElement>
    </packagedElement>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_allocation" client="_packageFeature" supplier="_part"/>`
	applications := `<sysml:Block xmi:id="_block" base_Class="_owner"/>
  <sysml:Allocate xmi:id="_allocate" base_Abstraction="_allocation"/>`
	r := migrateDocument(t, members, applications)
	assertStereotypeMigrationValid(t, r)
	if !strings.Contains(string(r.Notation), "dependency ") {
		t.Errorf("package-owned feature did not fall back to a dependency:\n%s", r.Notation)
	}
	if strings.Contains(string(r.Notation), "allocation def") {
		t.Errorf("package-owned feature was given a typed allocation end:\n%s", r.Notation)
	}
	es := entriesFor(r, "_allocation")
	if len(es) != 1 || es[0].Verdict != migrate.Approximated ||
		!strings.Contains(es[0].Note, "packageFeature") || !strings.Contains(es[0].Note, "feature of a package") {
		t.Errorf("allocation entry = %+v, want a noted package-feature fallback", es)
	}
}

func TestDirectedOperationKeepsItsDirection(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_owner" name="Owner">
      <ownedOperation xmi:type="uml:Operation" xmi:id="_provided" name="provided"/>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_required" name="required"/>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_both" name="both"/>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_plain" name="plain"/>
    </packagedElement>`
	applications := `
  <sysml:Block xmi:id="_block" base_Class="_owner"/>
  <sysml:DirectedFeature xmi:id="_providedDirection" base_Operation="_provided" featureDirection="provided"/>
  <sysml:DirectedFeature xmi:id="_requiredDirection" base_Operation="_required" featureDirection="required"/>
  <sysml:DirectedFeature xmi:id="_bothDirections" base_Operation="_both" featureDirection="providedRequired"/>`
	r := migrateDocument(t, members, applications)
	assertStereotypeMigrationValid(t, r)
	for _, line := range []string{
		"out action 'provided 2' : provided;",
		"in action 'required 2' : required;",
		"inout action 'both 2' : both;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantLine(t, r.Notation, "abstract action def plain;")
	wantLine(t, r.Notation, "action 'plain 2' : plain;")

	port := migrateDocument(t,
		`<packagedElement xmi:type="uml:Class" xmi:id="_port" name="Port">
           <ownedOperation xmi:type="uml:Operation" xmi:id="_provided" name="provided"/>
         </packagedElement>`,
		`<sysml:InterfaceBlock xmi:id="_interface" base_Class="_port"/>
         <sysml:DirectedFeature xmi:id="_direction" base_Operation="_provided" featureDirection="provided"/>`)
	assertStereotypeMigrationValid(t, port)
	wantLine(t, port.Notation, "out ref action 'provided 2' : provided;")
}

func TestInformationItemIsAnItemDef(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_represented" name="Represented"/>
    <packagedElement xmi:type="uml:InformationItem" xmi:id="_item" name="Payload" represented="_represented"/>`
	r := migrateDocument(t, members, "")
	assertStereotypeMigrationValid(t, r)
	wantLine(t, r.Notation, "item def Payload;")
	if strings.Contains(string(r.Notation), "Payload :> Represented") {
		t.Errorf("represented classifiers became generalizations:\n%s", r.Notation)
	}
	es := entriesFor(r, "_item")
	if len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("InformationItem entry = %+v, want Mapped", es)
	}
}

func TestItemFlowConveysItsItem(t *testing.T) {
	r := migrateDocument(t, flowModel, flowApplications)
	assertStereotypeMigrationValid(t, r)
	wantLine(t, r.Notation, "flow of Fuel from source.fuelOut to sink.fuelIn;")
	es := entriesFor(r, "_if")
	if len(es) != 1 || es[0].Verdict != migrate.Mapped {
		t.Errorf("item flow entry = %+v, want Mapped", es)
	}
}

func TestElementImportIsWritten(t *testing.T) {
	tests := []struct {
		name         string
		visibility   string
		alias        string
		want         string
		wantVerdict  migrate.Verdict
		wantNote     string
		extraMembers string
		extraApps    string
	}{
		{name: "public", want: "public import P::Thing;", wantVerdict: migrate.Mapped},
		{name: "private", visibility: ` visibility="private"`, want: "private import P::Thing;", wantVerdict: migrate.Mapped},
		{name: "alias", alias: ` alias="A"`, want: "alias A for P::Thing;", wantVerdict: migrate.Mapped},
		{name: "private alias", visibility: ` visibility="private"`, alias: ` alias="A"`, want: "private alias A for P::Thing;", wantVerdict: migrate.Mapped},
		{
			name:        "library skipped",
			wantVerdict: migrate.Skipped,
			wantNote:    "library element",
			extraMembers: `<packagedElement xmi:type="uml:Profile" xmi:id="_library" name="SysML">
                            <packagedElement xmi:type="uml:Class" xmi:id="_libraryTarget" name="LibraryType"/>
                          </packagedElement>`,
			extraApps: `<sysml:Block xmi:id="_libraryBlock" base_Class="_libraryTarget"/>`,
		},
		{
			name:         "name clash skipped",
			alias:        ` alias="Thing"`,
			wantVerdict:  migrate.Skipped,
			wantNote:     "Thing",
			extraMembers: `<packagedElement xmi:type="uml:Class" xmi:id="_existing" name="Thing"/>`,
			extraApps:    `<sysml:Block xmi:id="_existingBlock" base_Class="_existing"/>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := `_thing`
			if tt.name == "library skipped" {
				target = `_libraryTarget`
			}
			members := `
    <packagedElement xmi:type="uml:Package" xmi:id="_source" name="P">
      <packagedElement xmi:type="uml:Class" xmi:id="_thing" name="Thing"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Package" xmi:id="_importer" name="Q">
      <elementImport xmi:type="uml:ElementImport" xmi:id="_import" importedElement="` + target + `"` + tt.visibility + tt.alias + `/>
      ` + tt.extraMembers + `
    </packagedElement>`
			applications := `<sysml:Block xmi:id="_thingBlock" base_Class="_thing"/>` + tt.extraApps
			r := migrateDocument(t, members, applications)
			assertStereotypeMigrationValid(t, r)
			if tt.want != "" {
				wantLine(t, r.Notation, tt.want)
			}
			es := entriesFor(r, "_import")
			if len(es) != 1 || es[0].Verdict != tt.wantVerdict || (tt.wantNote != "" && !strings.Contains(es[0].Note, tt.wantNote)) {
				t.Errorf("ElementImport entry = %+v, want verdict %v and note containing %q", es, tt.wantVerdict, tt.wantNote)
			}
			if tt.wantVerdict == migrate.Skipped {
				for _, line := range []string{
					"public import P::Thing;",
					"private import P::Thing;",
					"alias Thing for P::Thing;",
					"import SysML::LibraryType;",
				} {
					if strings.Contains(string(r.Notation), line) {
						t.Errorf("skipped element import was written as %q:\n%s", line, r.Notation)
					}
				}
			}
		})
	}
}

func TestElementImportSkipsUnnamedProperty(t *testing.T) {
	const note = "the imported element is written without a name, so no import or alias can name it"
	for _, tt := range []struct {
		name  string
		alias string
	}{
		{name: "without alias"},
		{name: "with alias", alias: ` alias="A"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_owner" name="P">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_anonymous" type="_engine" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine"/>
    <packagedElement xmi:type="uml:Package" xmi:id="_importer" name="Q">
      <elementImport xmi:type="uml:ElementImport" xmi:id="_import" importedElement="_anonymous"` + tt.alias + `/>
    </packagedElement>`
			applications := `
  <sysml:Block xmi:id="_ownerBlock" base_Class="_owner"/>
  <sysml:Block xmi:id="_engineBlock" base_Class="_engine"/>`
			r := migrateDocument(t, members, applications)
			assertStereotypeMigrationValid(t, r)
			wantLine(t, r.Notation, "part : Engine;")

			es := entriesFor(r, "_import")
			if len(es) != 1 {
				t.Fatalf("ElementImport entries = %+v, want one skipped entry", es)
			}
			if es[0].Verdict != migrate.Skipped || es[0].Note != note {
				t.Errorf("ElementImport entry = %+v, want Skipped with note %q", es[0], note)
			}
			for _, line := range strings.Split(string(r.Notation), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "public import ") ||
					strings.HasPrefix(line, "private import ") ||
					strings.HasPrefix(line, "alias ") ||
					strings.HasPrefix(line, "private alias ") {
					t.Errorf("import of unnamed property was written: %q\n%s", line, r.Notation)
				}
			}
		})
	}
}

func TestElementImportReservesAliasNames(t *testing.T) {
	members := `
    <packagedElement xmi:type="uml:Class" xmi:id="_payloadType" name="Payload"/>
    <packagedElement xmi:type="uml:Package" xmi:id="_source" name="P">
      <packagedElement xmi:type="uml:Class" xmi:id="_thing" name="Thing"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Package" xmi:id="_importer" name="Q">
      <elementImport xmi:type="uml:ElementImport" xmi:id="_import" importedElement="_thing" alias="payload"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_anonymous" type="_payloadType"/>
      <packagedElement xmi:type="uml:Abstraction" xmi:id="_dependency" client="_anonymous" supplier="_thing"/>
    </packagedElement>`
	applications := `
  <sysml:Block xmi:id="_typeBlock" base_Class="_payloadType"/>
  <sysml:Block xmi:id="_thingBlock" base_Class="_thing"/>`
	r := migrateDocument(t, members, applications)
	assertStereotypeMigrationValid(t, r)
	wantLine(t, r.Notation, "alias payload for P::Thing;")
	if !strings.Contains(string(r.Notation), "payload2") {
		t.Errorf("fresh name reused the imported alias:\n%s", r.Notation)
	}
}

func TestPackageURIIsNotWritten(t *testing.T) {
	r := migrateDocument(t, `<packagedElement xmi:type="uml:Package" xmi:id="_package" name="WithURI" URI="https://example.test/package"/>`, "")
	assertStereotypeMigrationValid(t, r)
	if strings.Contains(string(r.Notation), "example.test") {
		t.Errorf("package URI was written:\n%s", r.Notation)
	}
}

func TestAllocationChainIsRelativeToItsDefinition(t *testing.T) {
	r := migrateDocument(t, ownerUsageAllocation, ownerUsageAllocationApplications)
	assertStereotypeMigrationValid(t, r)
	wantLine(t, r.Notation, "end :>> source : Ctl;")
	wantLine(t, r.Notation, "allocate source.run.'set status' to target;")
	wantNoLine(t, r.Notation, "allocate source.Ctl::run")
}
