package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// Every anonymous association is a connection def named after its end types,
// with one connection typed by it joining usages of the two types. An end a
// class owns crosses the property it is written as, reached through the
// opposite end; where that is unsound the def is still written, the end
// crosses nothing and the report says why. An end the association owns
// carries its multiplicity as the cross multiplicity.
func TestAnonymousAssociationEndsCrossTheirPropertiesWhereSound(t *testing.T) {
	r := migrateFixtureFile(t, "anonymous_associations")
	wantClean(t, "anonymous_associations.sysml", r)
	for _, line := range []string{
		"connection def WheelToCar {",
		"end wheels : Wheel crosses car.wheels;",
		"end car : Car crosses wheels.car;",
		"connection 'wheel to car' : WheelToCar connect wheel to car;",
		"connection def CarToGarage {",
		"end cars : Car crosses garage.cars;",
		"end garage : Garage crosses cars.garage;",
		"ref part cars : Car;",
		"connection 'car to garage' : CarToGarage connect car to garage;",
		"connection def DriverToCar {",
		"end driver : Driver crosses car.driver;",
		"end [0..*] ref car : Car;",
		"connection def ReadingToSource {",
		"end [0..*] ref reading : Reading;",
		"end source;",
		"connection def SinkToLogger {",
		"end sink;",
		"end logger : Logger;",
		"ref part Pump : Fleet::Pump;",
		"end Pump : Fleet::Pump crosses tank.Pump;",
		"end operator :> Operator;",
		"connection operates : Operates connect Operator to console;",
		"view Wheels {",
		"expose WheelToCar;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "connection 'operates 2'")
	wantNoLine(t, r.Notation, "private ref part cars")
	if got := strings.Count(string(r.Notation), " connect "); got != 5 {
		t.Errorf("connections = %d, want one for each def whose end types have usages\n%s", got, r.Notation)
	}
	wantNote(t, r, "_carWheel", migrate.Approximated, "the anonymous Association is written as connection def WheelToCar; its one usage is the connection wheel to car joining wheel and car")
	wantNote(t, r, "_carDriverAssoc", migrate.Approximated, "its one usage is the connection driver to car joining driver and car")
	wantNote(t, r, "_garageCars", migrate.Approximated, "private visibility is not written: the connection def of the association (_garageCarAssoc) in Fleet crosses it")
	wantNote(t, r, "_sensorReadingAssoc", migrate.Approximated, "end reading crosses no property: the opposite end is untyped, so the property cannot be reached through it")
	wantNote(t, r, "_sensorReadingAssoc", migrate.Approximated, "no connection usage joins usages of its end types: end source is untyped")
	wantNote(t, r, "_loggerSinkAssoc", migrate.Approximated, "end sink crosses no property: the end's type is not written")
	wantNote(t, r, "_loggerSinkAssoc", migrate.Approximated, "no connection usage joins usages of its end types")
	wantNote(t, r, "_tankPumpAssoc", migrate.Approximated, "the anonymous Association is written as connection def PumpToTank; its one usage is the connection pump to tank joining pump and tank")
	wantNote(t, r, "_operates", migrate.Approximated, "end operator crosses no property: its type Fleet::Operator is written as a usage, not a definition")
	for _, id := range []string{"_sensorReadingAssoc", "_loggerSinkAssoc", "_tankPumpAssoc", "_operates"} {
		if es := entriesFor(r, id); len(es) != 1 || strings.Contains(es[0].Note, "member-end properties") {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
}

func TestAnonymousAssociationEndChainsToNestedViewFeature(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Package" xmi:id="_pkg" name="Pkg">
      <packagedElement xmi:type="uml:Class" xmi:id="_cal" name="Calibration">
        <nestedClassifier xmi:type="uml:Class" xmi:id="_daily" name="Daily Cal">
          <nestedClassifier xmi:type="uml:Class" xmi:id="_dark" name="Dark Frame"/>
          <ownedAttribute xmi:type="uml:Property" xmi:id="_dark_frame" name="dark frame" type="_dark" association="_assoc"/>
        </nestedClassifier>
        <packagedElement xmi:type="uml:Association" xmi:id="_assoc" memberEnd="_dark_frame _daily_end">
          <ownedEnd xmi:type="uml:Property" xmi:id="_daily_end" name="daily Cal" type="_daily" association="_assoc"/>
        </packagedElement>
      </packagedElement>
    </packagedElement>`, `
  <sysml:View xmi:id="_cal_view" base_Class="_cal"/>
  <sysml:View xmi:id="_daily_view" base_Class="_daily"/>
  <sysml:View xmi:id="_dark_view" base_Class="_dark"/>`)
	wantClean(t, "anonymous_association_view.sysml", r)
	wantLine(t, r.Notation, "end 'daily Cal' :> Pkg::Calibration.'Daily Cal';")
}

func TestAnonymousAssociationEndChainsToNestedRequirement(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Package" xmi:id="_pkg" name="Pkg">
      <packagedElement xmi:type="uml:Class" xmi:id="_payload" name="Payload"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_req" name="Calibration">
        <nestedClassifier xmi:type="uml:Class" xmi:id="_daily" name="Daily Requirement"/>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_payload_end" name="payload" type="_payload" association="_assoc"/>
        <packagedElement xmi:type="uml:Association" xmi:id="_assoc" memberEnd="_payload_end _daily_end">
          <ownedEnd xmi:type="uml:Property" xmi:id="_daily_end" name="Daily Requirement" type="_daily" association="_assoc"/>
        </packagedElement>
      </packagedElement>
    </packagedElement>`, `
  <sysml:Block xmi:id="_payload_block" base_Class="_payload"/>
  <sysml:Requirement xmi:id="_req_outer" base_Class="_req" Id="R1" Text="The system shall be calibrated."/>
  <sysml:Requirement xmi:id="_req_daily" base_Class="_daily" Id="R1.1" Text="The system shall be calibrated daily."/>`)
	wantClean(t, "anonymous_association_requirement.sysml", r)
	wantLine(t, r.Notation, "end 'Daily Requirement' :> Pkg::Calibration.'Daily Requirement';")
}

func TestTableQueriesResolveEscapedNames(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Package" xmi:id="_pkg" name="Frank's Braindump">
      <packagedElement xmi:type="uml:Package" xmi:id="_composition" name="Composition Example">
        <packagedElement xmi:type="uml:Class" xmi:id="_instrument" name="Instrument">
          <ownedAttribute xmi:type="uml:Property" xmi:id="_bench_part" name="bench" type="_bench" aggregation="composite" association="_association"/>
        </packagedElement>
        <packagedElement xmi:type="uml:Class" xmi:id="_bench" name="Optical&#10;Bench"/>
        <packagedElement xmi:type="uml:Association" xmi:id="_association" memberEnd="_bench_part _instrument_end">
          <ownedEnd xmi:type="uml:Property" xmi:id="_instrument_end" name="instrument" type="_instrument" association="_association"/>
        </packagedElement>
      </packagedElement>
    </packagedElement>`, `
  <xmi:Extension extender="Example UML Tool 1.0">
    <modelExtension>
      <ownedDiagram xmi:type="uml:Diagram" xmi:id="_property_diagram" name="Properties" ownerOfDiagram="_composition">
        <xmi:Extension extender="Example UML Tool 1.0">
          <diagramRepresentation>
            <diagram:DiagramRepresentationObject xmlns:diagram="http://www.example.com/tool/diagram" type="Generic Table" umlType="Class Diagram">
              <diagramContents/>
            </diagram:DiagramRepresentationObject>
          </diagramRepresentation>
        </xmi:Extension>
      </ownedDiagram>
      <ownedDiagram xmi:type="uml:Diagram" xmi:id="_instance_diagram" name="Bench Instances" ownerOfDiagram="_composition">
        <xmi:Extension extender="Example UML Tool 1.0">
          <diagramRepresentation>
            <diagram:DiagramRepresentationObject xmlns:diagram="http://www.example.com/tool/diagram" type="SysML Instance Table" umlType="Class Diagram">
              <diagramContents/>
            </diagram:DiagramRepresentationObject>
          </diagramRepresentation>
        </xmi:Extension>
      </ownedDiagram>
    </modelExtension>
  </xmi:Extension>
  <MagicDraw_Profile:DiagramTable xmlns:MagicDraw_Profile="http://www.omg.org/spec/UML/20131001/MagicDrawProfile" xmi:id="_property_table" base_Diagram="_property_diagram" scope="_composition" excludedElements="_instrument_end">
    <rowElementType href="http://www.omg.org/spec/UML/20131001/UML.xmi#Property"/>
    <columnIds>QPROP:Element:qualifiedName</columnIds>
  </MagicDraw_Profile:DiagramTable>
  <MagicDraw_Profile:InstanceTable xmlns:MagicDraw_Profile="http://www.omg.org/spec/UML/20131001/MagicDrawProfile" xmi:id="_instance_table" base_Diagram="_instance_diagram" scope="_composition" includeSubtypesOfRowTypes="true" showScopeAsRoot="false">
    <classifiers xmi:idref="_bench"/>
    <columnIds>QPROP:Element:qualifiedName</columnIds>
  </MagicDraw_Profile:InstanceTable>`)
	wantClean(t, "escaped_table_names.sysml", r)
	s := session(t, r)
	propertyRows := rows(t, s, `Model::'Frank\'s Braindump'::'Composition Example'::'Properties Rows'`)
	if !strings.Contains(propertyRows, `Model::Frank\\'s Braindump::Composition Example::Instrument::bench`) {
		t.Errorf("Properties rows omit the class-owned part property:\n%s", propertyRows)
	}
	if strings.Contains(propertyRows, "::OpticalBenchToInstrument::instrument") {
		t.Errorf("Properties rows include the excluded association-owned end:\n%s", propertyRows)
	}

	instanceRows := rows(t, s, `Model::'Frank\'s Braindump'::'Composition Example'::'Bench Instances Rows'`)
	if strings.Contains(instanceRows, "::bench") {
		t.Errorf("Bench Instances rows include the synthesized connection usage:\n%s", instanceRows)
	}

	notation := string(r.Notation)
	if !strings.Contains(notation, `Model::Frank\\'s Braindump::Composition Example::OpticalBenchToInstrument::instrument`) {
		t.Errorf("the excluded row query does not spell the connection-def path with escaped names:\n%s", notation)
	}
	for _, document := range []string{
		`Model::'Frank\'s Braindump'::'Composition Example'::'Properties Document'`,
		`Model::'Frank\'s Braindump'::'Composition Example'::'Bench Instances Document'`,
	} {
		page := html(t, s, document)
		if strings.Contains(page, "names no single element") {
			t.Errorf("%s rendered an error page:\n%s", document, page)
		}
	}
}
