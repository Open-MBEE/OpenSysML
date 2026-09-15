package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

const realHref = `<type href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML_dataType.Real"/>`
const integerHref = `<type href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML_dataType.Integer"/>`
const booleanHref = `<type href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML_dataType.Boolean"/>`

func wantNote(t *testing.T, r *migrate.Result, id string, verdict migrate.Verdict, note string) {
	t.Helper()
	es := entriesFor(r, id)
	if len(es) != 1 || es[0].Verdict != verdict || !strings.Contains(es[0].Note, note) {
		t.Errorf("entries for %s = %+v, want one %v entry noting %q", id, es, verdict, note)
	}
}

func wantClean(t *testing.T, name string, r *migrate.Result) {
	t.Helper()
	for _, d := range errors(t, name, r.Notation) {
		t.Errorf("%v", d)
	}
}

// An instance of a value type is a value, not an occurrence: it is written as
// an attribute usage typed by the value type, holding its slots. An instance
// classified by both a block and a value type is the individual alone.
func TestInstanceOfValueTypeIsAnAttributeUsage(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:DataType" xmi:id="_pos" name="Position">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x">`+realHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Rover"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_home" name="home" classifier="_pos">
      <slot xmi:id="_s" definingFeature="_x">
        <value xmi:type="uml:LiteralReal" xmi:id="_v" value="2.5"/>
      </slot>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_mixed" name="mixed" classifier="_b _pos"/>`,
		`<sysml:ValueType xmi:id="_st" base_DataType="_pos"/><sysml:Block xmi:id="_sb" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute home : Position {")
	wantLine(t, r.Notation, "attribute :>> x = 2.5;")
	wantLine(t, r.Notation, "individual def mixed :> Rover;")
	wantNote(t, r, "_home", migrate.Mapped, "")
	wantNote(t, r, "_mixed", migrate.Approximated, "an individual cannot specialize a value type")
	wantClean(t, "value.sysml", r)
}

// A tool stores a typed-in default as a string, or a whole number as a real;
// the literal takes the scalar type of the feature that holds it.
func TestTypedInLiteralsTakeTheFeaturesScalarType(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_gain" name="gain">`+realHref+`
        <defaultValue xmi:type="uml:LiteralString" xmi:id="_gv" value="17"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_poles" name="poles">`+integerHref+`
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_pv" value="4.0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_on" name="on">`+booleanHref+`
        <defaultValue xmi:type="uml:LiteralString" xmi:id="_ov" value="true"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_label" name="label">`+realHref+`
        <defaultValue xmi:type="uml:LiteralString" xmi:id="_lv" value="fast"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute gain : ScalarValues::Real default = 17.0;")
	wantLine(t, r.Notation, "attribute poles : ScalarValues::Integer default = 4;")
	wantLine(t, r.Notation, "attribute on : ScalarValues::Boolean default = true;")
	wantLine(t, r.Notation, `attribute label : ScalarValues::Real default = "fast";`)
	wantNote(t, r, "_gain", migrate.Approximated, `the string "17" is written as the Real the feature holds`)
	wantNote(t, r, "_poles", migrate.Approximated, "the real 4.0 is written as the Integer the feature holds")
	wantNote(t, r, "_label", migrate.Mapped, "")
}

// A literal is no value of a value type or enumeration with no scalar base:
// such a default is left as a comment rather than a binding no v2 checker
// accepts. A value type over an external base is written with none, so the
// same holds there.
func TestLiteralOnStructuredValueTypeIsNotBound(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:DataType" xmi:id="_map" name="Map"/>
    <packagedElement xmi:type="uml:DataType" xmi:id="_ext" name="Ext">
      <generalization xmi:id="_g" xmi:type="uml:Generalization"><general href="Other.xmi#_base"/></generalization>
    </packagedElement>
    <packagedElement xmi:type="uml:Enumeration" xmi:id="_mode" name="Mode">
      <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_fast" name="fast"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Sensor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ref" name="refMap" type="_map">
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_rv" value="0.0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_e" name="e" type="_ext">
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_ev" value="0.0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_md" name="mode" type="_mode">
        <defaultValue xmi:type="uml:LiteralString" xmi:id="_mdv" value="fast"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:ValueType xmi:id="_s1" base_DataType="_map"/><sysml:ValueType xmi:id="_s2" base_DataType="_ext"/><sysml:Block xmi:id="_st" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute refMap : Map {")
	wantLine(t, r.Notation, "attribute e : Ext {")
	wantLine(t, r.Notation, "attribute mode : Mode {")
	wantNoLine(t, r.Notation, "default =")
	wantNote(t, r, "_ref", migrate.Approximated, "the literal 0.0 is not a value of Map, which has no scalar base")
	wantNote(t, r, "_md", migrate.Approximated, `the literal "fast" is not a value of Mode, which has no scalar base`)
	wantClean(t, "structured.sysml", r)
}

// A slot whose values contradict its feature — more than the multiplicity
// admits, or a repeat on a unique feature — is invalid in both languages and
// is left as a comment naming the values.
func TestSlotContradictingItsFeatureIsUnmapped(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:DataType" xmi:id="_cal" name="Calibration">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_t" name="t">`+realHref+`
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_tl"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_tu" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_pix" name="pix">`+integerHref+`
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_pl" value="3"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_pu" value="3"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bag" name="bag" isUnique="false">`+realHref+`
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_bl"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_bu" value="*"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i" name="cal" classifier="_cal">
      <slot xmi:id="_st" definingFeature="_t">
        <value xmi:type="uml:LiteralReal" xmi:id="_v1" value="815.0"/>
        <value xmi:type="uml:LiteralReal" xmi:id="_v2" value="815.0"/>
      </slot>
      <slot xmi:id="_sp" definingFeature="_pix">
        <value xmi:type="uml:LiteralInteger" xmi:id="_v3" value="1"/>
      </slot>
      <slot xmi:id="_sb" definingFeature="_bag">
        <value xmi:type="uml:LiteralReal" xmi:id="_v4" value="1.0"/>
        <value xmi:type="uml:LiteralReal" xmi:id="_v5" value="1.0"/>
      </slot>
    </packagedElement>`, `<sysml:ValueType xmi:id="_s1" base_DataType="_cal"/>`)
	wantLine(t, r.Notation, "attribute :>> bag = (1.0, 1.0);")
	wantNoLine(t, r.Notation, "attribute :>> t")
	wantNoLine(t, r.Notation, "attribute :>> pix")
	wantNote(t, r, "_st", migrate.Unmapped, "the slot repeats the value 815.0 on a unique feature; its values are 815.0, 815.0")
	wantNote(t, r, "_sp", migrate.Unmapped, "the slot holds 1 value(s) for a feature of multiplicity 3")
	wantNote(t, r, "_sb", migrate.Mapped, "")
	wantClean(t, "slots.sysml", r)
}

// A slot of a feature no classifier of the instance has is not written.
func TestSlotOfForeignFeatureIsUnmapped(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:DataType" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x">`+realHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:DataType" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_y" name="y">`+realHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i" name="a" classifier="_a">
      <slot xmi:id="_s" definingFeature="_y">
        <value xmi:type="uml:LiteralReal" xmi:id="_v" value="1.0"/>
      </slot>
    </packagedElement>`, `<sysml:ValueType xmi:id="_s1" base_DataType="_a"/><sysml:ValueType xmi:id="_s2" base_DataType="_b"/>`)
	wantNoLine(t, r.Notation, ":>> y")
	wantNote(t, r, "_s", migrate.Unmapped, "the slot's defining feature B::y is not a feature of any classifier of the instance")
	wantClean(t, "foreign.sysml", r)
}

// A default that names an individual types the usage by that individual,
// since a v2 definition is not an expression.
func TestIndividualDefaultTypesTheUsage(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_tab" name="Table"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_std" name="usual" classifier="_tab"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Procedure">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="angles" type="_tab" aggregation="composite">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_l"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_u" value="*"/>
        <defaultValue xmi:type="uml:InstanceValue" xmi:id="_dv" instance="_std"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_tab"/><sysml:Block xmi:id="_s2" base_Class="_b"/>`)
	wantLine(t, r.Notation, "part angles : Table, usual[0..*];")
	wantNoLine(t, r.Notation, "default = usual")
	wantNote(t, r, "_p", migrate.Approximated, "the default value, the individual usual, is written as a type of the usage")
	wantClean(t, "individual.sysml", r)
}

// A value type with a unit or quantity kind and no base is a magnitude,
// written over ScalarValues::Real so its values can be numbers.
func TestQuantityValueTypeIsReal(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_kg" name="kilogram"/>
    <packagedElement xmi:type="uml:DataType" xmi:id="_mass" name="Mass"/>
    <packagedElement xmi:type="uml:DataType" xmi:id="_plain" name="Plain"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Rover">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_m" name="m" type="_mass">
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_mv" value="900.0"/>
      </ownedAttribute>
    </packagedElement>`, `
  <sysml:Unit xmi:id="_su" base_InstanceSpecification="_kg"/>
  <sysml:ValueType xmi:id="_s1" base_DataType="_mass" unit="_kg"/>
  <sysml:ValueType xmi:id="_s2" base_DataType="_plain"/>
  <sysml:Block xmi:id="_s3" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute def Mass :> ScalarValues::Real {")
	wantLine(t, r.Notation, "attribute def Plain;")
	wantLine(t, r.Notation, "attribute m : Mass default = 900.0;")
	wantNote(t, r, "_mass", migrate.Approximated, "a value type with a unit or quantity kind and no base type is written as ScalarValues::Real")
	wantClean(t, "quantity.sysml", r)
}

// An interface block's undirected item is written as a reference, as a port's
// nested usages other than ports cannot be composite; a directed flow property
// stays a plain item.
func TestUndirectedItemInAnInterfaceBlockIsAReference(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_if" name="FuelInterface">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_flow" name="fuel" type="_fuel" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_spare" name="spare" type="_fuel" aggregation="composite"/>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_fuel"/>
  <sysml:InterfaceBlock xmi:id="_s2" base_Class="_if"/>
  <sysml:FlowProperty xmi:id="_s3" base_Property="_flow" direction="in"/>`)
	wantLine(t, r.Notation, "in item fuel : Fuel;")
	wantLine(t, r.Notation, "ref item spare : Fuel;")
	wantNote(t, r, "_spare", migrate.Approximated, "the undirected item of an interface block is written as a reference")
	wantClean(t, "interface.sysml", r)
}

// A private feature a connector in another block reaches through a nested
// path is written without its visibility, so the connector can name it.
func TestFeatureReachedByAConnectorLosesItsPrivacy(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_in" name="intake" type="_fuel" visibility="private"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_aux" name="aux" type="_fuel" visibility="private"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_engine" name="engine" type="_engine" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_out" name="tank" type="_fuel"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_conn" name="feed">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_pt_out"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_pt_in" partWithPort="_p_engine"/>
      </ownedConnector>
    </packagedElement>`, `
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_fuel"/>
  <sysml:Block xmi:id="_s2" base_Class="_engine"/>
  <sysml:Block xmi:id="_s3" base_Class="_car"/>`)
	wantLine(t, r.Notation, "port intake : Fuel;")
	wantLine(t, r.Notation, "private port aux : Fuel;")
	wantLine(t, r.Notation, "connection feed connect tank to engine.intake;")
	wantNote(t, r, "_pt_in", migrate.Approximated, "private visibility is not written: connector 'feed' in Car reaches it")
	wantClean(t, "reached.sysml", r)
}

// A nested path whose segment is no feature of the type before it cannot be
// written; the connector is left behind naming the segment.
func TestConnectorPathSegmentMustBelongToThePrecedingType(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_in" name="intake" type="_fuel"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_pump" name="inlet" type="_fuel"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_engine" name="engine" type="_engine" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_out" name="tank" type="_fuel"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_conn" name="feed">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_pt_out"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_pt_pump"/>
      </ownedConnector>
    </packagedElement>`, `
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_fuel"/>
  <sysml:Block xmi:id="_s2" base_Class="_engine"/>
  <sysml:Block xmi:id="_s3" base_Class="_pump"/>
  <sysml:Block xmi:id="_s4" base_Class="_car"/>
  <sysml:NestedConnectorEnd xmi:id="_nce" base_ConnectorEnd="_e2">
    <propertyPath xmi:idref="_p_engine"/>
  </sysml:NestedConnectorEnd>`)
	wantNoLine(t, r.Notation, "connect ")
	wantNote(t, r, "_conn", migrate.Unmapped, "the connector end's role Pump::inlet is not a feature of Engine")
	wantClean(t, "segment.sysml", r)
}

// A specializing block's property named like an inherited one redefines it:
// UML lets the names clash, v2 does not, and the intent is the redefinition.
func TestSameNamedPropertyRedefinesTheInheritedOne(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:DataType" xmi:id="_ps" name="PortStatus"/>
    <packagedElement xmi:type="uml:DataType" xmi:id="_sw" name="Switch">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_dl" name="downlink" type="_ps"/>
    </packagedElement>
    <packagedElement xmi:type="uml:DataType" xmi:id="_m1" name="M1Switch">
      <generalization xmi:id="_g" xmi:type="uml:Generalization" general="_sw"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_dl2" name="downlink" type="_ps">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_l" value="6"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_u" value="6"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:ValueType xmi:id="_s1" base_DataType="_ps"/><sysml:ValueType xmi:id="_s2" base_DataType="_sw"/><sysml:ValueType xmi:id="_s3" base_DataType="_m1"/>`)
	wantLine(t, r.Notation, "attribute downlink : PortStatus[6] :>> downlink;")
	wantNote(t, r, "_dl2", migrate.Approximated, "written as a redefinition of the inherited Switch::downlink")
	wantClean(t, "shadow.sysml", r)
}
