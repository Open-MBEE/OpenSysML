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
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_mixed" name="mixed" classifier="_b _pos">
      <slot xmi:id="_ms" definingFeature="_x">
        <value xmi:type="uml:LiteralReal" xmi:id="_mv" value="1.0"/>
      </slot>
    </packagedElement>`,
		`<sysml:ValueType xmi:id="_st" base_DataType="_pos"/><sysml:Block xmi:id="_sb" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute home : Position {")
	wantLine(t, r.Notation, "attribute :>> x = 2.5;")
	wantLine(t, r.Notation, "individual part def mixed :> Rover {")
	wantNoLine(t, r.Notation, "x = 1.0")
	wantNote(t, r, "_home", migrate.Mapped, "")
	wantNote(t, r, "_mixed", migrate.Approximated, "an individual cannot specialize a value type")
	wantNote(t, r, "_ms", migrate.Unmapped, "the slot's defining feature Position::x is not a feature of any classifier the instance is written to specialize")
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
    </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute gain : ScalarValues::Real default = 17.0;")
	wantLine(t, r.Notation, "attribute poles : ScalarValues::Integer default = 4;")
	wantLine(t, r.Notation, "attribute on : ScalarValues::Boolean default = true;")
	wantNote(t, r, "_gain", migrate.Approximated, `the string "17" is written as the Real the feature holds`)
	wantNote(t, r, "_poles", migrate.Approximated, "the real 4.0 is written as the Integer the feature holds")
	wantClean(t, "typed.sysml", r)
}

// A literal that spells no value of the feature's scalar type is left as a
// comment: a binding of it would fail every v2 checker.
func TestLiteralOfAnotherScalarTypeIsNotBound(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_label" name="label">`+realHref+`
        <defaultValue xmi:type="uml:LiteralString" xmi:id="_lv" value="fast"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_poles" name="poles">`+integerHref+`
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_pv" value="4.5"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_on" name="on">`+booleanHref+`
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_ov" value="1"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_gain" name="gain">`+realHref+`
        <defaultValue xmi:type="uml:LiteralBoolean" xmi:id="_gv" value="true"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rate" name="rate">`+realHref+`
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_rv" value="3"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_serial" name="serial">`+integerHref+`
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_sv" value="9007199254740993.0"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute label : ScalarValues::Real {")
	wantLine(t, r.Notation, "attribute poles : ScalarValues::Integer {")
	wantLine(t, r.Notation, "attribute on : ScalarValues::Boolean {")
	wantLine(t, r.Notation, "attribute gain : ScalarValues::Real {")
	wantLine(t, r.Notation, "attribute rate : ScalarValues::Real default = 3;")
	wantNoLine(t, r.Notation, `default = "fast"`)
	wantNoLine(t, r.Notation, "default = 4.5")
	wantNoLine(t, r.Notation, "default = 1;")
	wantNoLine(t, r.Notation, "default = true")
	wantNote(t, r, "_label", migrate.Approximated, `default value not migrated: the string "fast" is not a value of Real, which the feature holds`)
	wantNote(t, r, "_poles", migrate.Approximated, "default value not migrated: the real 4.5 is not a value of Integer, which the feature holds")
	wantNote(t, r, "_on", migrate.Approximated, "default value not migrated: the integer 1 is not a value of Boolean, which the feature holds")
	wantNote(t, r, "_gain", migrate.Approximated, "default value not migrated: the boolean true is not a value of Real, which the feature holds")
	wantNote(t, r, "_rate", migrate.Mapped, "")
	wantLine(t, r.Notation, "attribute serial : ScalarValues::Integer default = 9007199254740993;")
	wantClean(t, "mistyped.sysml", r)
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

// Slot values repeat by value, as the analyzer reads them: 1 and 1.0 are one
// number spelled twice; "1" and "1.0" are two strings.
func TestSlotValuesRepeatByValue(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:DataType" xmi:id="_cal" name="Calibration">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_t" name="t">`+realHref+`
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_tl"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_tu" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_tag" name="tag">
        <type href="http://www.omg.org/spec/UML/20161101/PrimitiveTypes.xmi#String"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_gl"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_gu" value="*"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i" name="cal" classifier="_cal">
      <slot xmi:id="_st" definingFeature="_t">
        <value xmi:type="uml:LiteralInteger" xmi:id="_v1" value="1"/>
        <value xmi:type="uml:LiteralReal" xmi:id="_v2" value="1.0"/>
      </slot>
      <slot xmi:id="_sg" definingFeature="_tag">
        <value xmi:type="uml:LiteralString" xmi:id="_v3" value="1"/>
        <value xmi:type="uml:LiteralString" xmi:id="_v4" value="1.0"/>
      </slot>
    </packagedElement>`, `<sysml:ValueType xmi:id="_s1" base_DataType="_cal"/>`)
	wantLine(t, r.Notation, `attribute :>> tag = ("1", "1.0");`)
	wantNoLine(t, r.Notation, "attribute :>> t ")
	wantNote(t, r, "_st", migrate.Unmapped, "the slot repeats the value 1.0 on a unique feature; its values are 1, 1.0")
	wantNote(t, r, "_sg", migrate.Mapped, "")
	wantClean(t, "repeats.sysml", r)
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
	wantNote(t, r, "_s", migrate.Unmapped, "the slot's defining feature B::y is not a feature of any classifier the instance is written to specialize")
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

// An untyped property whose default is an individual is typed by that
// individual alone: the instance is the only classification it has.
func TestUntypedPropertyIsTypedByItsIndividualDefault(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_tab" name="Table"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_std" name="usual" classifier="_tab"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Procedure">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="angles">
        <defaultValue xmi:type="uml:InstanceValue" xmi:id="_dv" instance="_std"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_tab"/><sysml:Block xmi:id="_s2" base_Class="_b"/>`)
	wantLine(t, r.Notation, "ref angles : usual;")
	wantNoLine(t, r.Notation, "default value not migrated")
	wantNote(t, r, "_p", migrate.Approximated, "the default value, the individual usual, is written as a type of the usage")
	wantClean(t, "untyped-individual.sysml", r)
}

// An individual types a usage only when it is of the usage's kind: an
// interface block's individual is no port def, a constraint block's no part
// def, so such a default stays a comment.
func TestIndividualDefaultOfAnotherKindIsNotAType(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_bus" name="Bus"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_fits" name="Fits"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_b1" name="bus 1" classifier="_bus"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_f1" name="fits 1" classifier="_fits"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_v" name="Vehicle">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_p" name="bus" type="_bus">
        <defaultValue xmi:type="uml:InstanceValue" xmi:id="_dv" instance="_b1"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_c" name="check" type="_fits" aggregation="composite">
        <defaultValue xmi:type="uml:InstanceValue" xmi:id="_dc" instance="_f1"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_u" name="loose">
        <defaultValue xmi:type="uml:InstanceValue" xmi:id="_du" instance="_b1"/>
      </ownedAttribute>
    </packagedElement>`, `
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_bus"/>
  <sysml:Block xmi:id="_s2" base_Class="_fits"/>
  <sysml:Block xmi:id="_s3" base_Class="_v"/>`)
	wantLine(t, r.Notation, "port bus : Bus {")
	wantLine(t, r.Notation, "part check : Fits, 'fits 1';")
	wantLine(t, r.Notation, "ref loose : 'bus 1';")
	wantNote(t, r, "_p", migrate.Approximated, "default value not migrated")
	wantClean(t, "kinds-default.sysml", r)
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

// A connector left as a comment reaches nothing: the private feature its
// other, resolvable end names keeps its visibility.
func TestUnmappedConnectorDoesNotExposeItsResolvableEnd(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_fuel" name="Fuel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_in" name="intake" type="_fuel" visibility="private"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt_pump" name="inlet" type="_fuel"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_engine" name="engine" type="_engine" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_conn" name="feed">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e1" role="_pt_in" partWithPort="_p_engine"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_e2" role="_pt_pump"/>
      </ownedConnector>
    </packagedElement>`, `
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_fuel"/>
  <sysml:Block xmi:id="_s2" base_Class="_engine"/>
  <sysml:Block xmi:id="_s3" base_Class="_pump"/>
  <sysml:Block xmi:id="_s4" base_Class="_car"/>`)
	wantLine(t, r.Notation, "private port intake : Fuel;")
	wantNoLine(t, r.Notation, "connect ")
	wantNote(t, r, "_conn", migrate.Unmapped, "the connector end's role Pump::inlet is not a feature of Car")
	wantNote(t, r, "_pt_in", migrate.Mapped, "")
	wantClean(t, "unreached.sysml", r)
}

// A slot left as a comment reaches nothing either: only a slot that is written
// takes the visibility off its private defining feature.
func TestUnmappedSlotDoesNotExposeItsDefiningFeature(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_mcs" name="MCS"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_tmt" name="TMT">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="mcs" type="_mcs" aggregation="composite" visibility="private"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_q" name="spare" type="_mcs" aggregation="composite" visibility="private"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_m1" name="mcs 1" classifier="_mcs"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_t1" name="tmt 1" classifier="_tmt">
      <slot xmi:type="uml:Slot" xmi:id="_sl1" definingFeature="_p">
        <value xmi:type="uml:InstanceValue" xmi:id="_v1" instance="_m1"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_sl2" definingFeature="_q">
        <value xmi:type="uml:LiteralInteger" xmi:id="_v2" value="3"/>
      </slot>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_mcs"/>
  <sysml:Block xmi:id="_s2" base_Class="_tmt"/>`)
	wantLine(t, r.Notation, "part mcs : MCS;")
	wantLine(t, r.Notation, "private part spare : MCS;")
	wantLine(t, r.Notation, "individual part :>> mcs : 'mcs 1';")
	wantNote(t, r, "_p", migrate.Approximated, "private visibility is not written: instance 'tmt 1' has a slot for it")
	wantNote(t, r, "_q", migrate.Mapped, "")
	wantNote(t, r, "_sl2", migrate.Unmapped, "a part holds instances; the slot's value is a LiteralInteger")
	wantClean(t, "unreached-slot.sysml", r)
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

// A port sharing the name of an inherited value property is another kind of
// usage, so it cannot redefine it: the collision is reported, not papered over.
func TestSameNamedPortCannotRedefineAnInheritedProperty(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:DataType" xmi:id="_ps" name="PortStatus"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_if" name="Link"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="Radio">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_st" name="status" type="_ps"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Radio2">
      <generalization xmi:id="_g" xmi:type="uml:Generalization" general="_a"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_pt" name="status" type="_if"/>
    </packagedElement>`, `<sysml:ValueType xmi:id="_s1" base_DataType="_ps"/><sysml:InterfaceBlock xmi:id="_s2" base_Class="_if"/><sysml:Block xmi:id="_s3" base_Class="_a"/><sysml:Block xmi:id="_s4" base_Class="_b"/>`)
	wantLine(t, r.Notation, "port status : Link;")
	wantNoLine(t, r.Notation, ":>> status")
	wantNote(t, r, "_pt", migrate.Approximated, "shares the name of the inherited Radio::status, which is written as attribute and so cannot be redefined by this port")
	wantClean(t, "shadow-port.sysml", r)
}

// A slot of a part property holds instances: one redefines the part as an
// individual typed by that instance, several each subset it under a
// redefinition counting them. A reference part keeps its `ref`.
func TestPartSlotsRedefineThePartByItsInstance(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_mcs" name="MCS"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_fast" name="FastMCS">
      <generalization xmi:type="uml:Generalization" xmi:id="_gen" general="_mcs"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_tmt" name="TMT">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="mcs" type="_mcs" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_q" name="spares" type="_mcs" aggregation="composite">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_lo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_up" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_s" name="shared" type="_mcs" aggregation="shared"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_m1" name="mcs 1" classifier="_fast"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_m2" name="mcs 2" classifier="_mcs"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_t1" name="tmt 1" classifier="_tmt">
      <slot xmi:type="uml:Slot" xmi:id="_sl1" definingFeature="_p">
        <value xmi:type="uml:InstanceValue" xmi:id="_v1" instance="_m1"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_sl2" definingFeature="_q">
        <value xmi:type="uml:InstanceValue" xmi:id="_v2" instance="_m1"/>
        <value xmi:type="uml:InstanceValue" xmi:id="_v3" instance="_m2"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_sl3" definingFeature="_s">
        <value xmi:type="uml:InstanceValue" xmi:id="_v4" instance="_m2"/>
      </slot>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_t2" name="tmt 2" classifier="_tmt">
      <slot xmi:type="uml:Slot" xmi:id="_sl4" definingFeature="_q">
        <value xmi:type="uml:InstanceValue" xmi:id="_v5" instance="_m2"/>
      </slot>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_mcs"/>
  <sysml:Block xmi:id="_s2" base_Class="_fast"/>
  <sysml:Block xmi:id="_s3" base_Class="_tmt"/>`)
	wantLine(t, r.Notation, "individual part def 'mcs 1' :> FastMCS;")
	wantLine(t, r.Notation, "individual part def 'tmt 1' :> TMT {")
	wantLine(t, r.Notation, "individual part :>> mcs : 'mcs 1';")
	wantLine(t, r.Notation, "part :>> spares [2];")
	wantLine(t, r.Notation, "individual part : 'mcs 1' :> spares;")
	wantLine(t, r.Notation, "individual part : 'mcs 2' :> spares;")
	wantLine(t, r.Notation, "ref individual part :>> shared : 'mcs 2';")
	wantLine(t, r.Notation, "individual part :>> spares : 'mcs 2'[1];")
	for _, id := range []string{"_sl1", "_sl2", "_sl3", "_sl4"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("entries for %s = %+v", id, es)
		}
	}
	wantClean(t, "part-slots.sysml", r)
}

// A part slot whose value is not an individual of the part's type has no v2
// form: a literal, an instance without a classifier, an instance of another
// block or of a block where an item is due, or a port's slot, since v2 has no
// individual port def for an instance of an interface block to be.
func TestPartSlotWithoutAConformingIndividualIsUnmapped(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_mcs" name="MCS"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_other" name="Other"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_if" name="Bus"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_tmt" name="TMT">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="mcs" type="_mcs" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="bus" type="_if" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_u" name="loose"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_port" name="link" type="_if"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_pg" name="ping" type="_sig" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_sig" name="Ping"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_o1" name="other 1" classifier="_other"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_bare" name="snapshot"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_bus1" name="bus 1" classifier="_if"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_m1" name="mcs 1" classifier="_mcs"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_t1" name="tmt 1" classifier="_tmt">
      <slot xmi:type="uml:Slot" xmi:id="_sl1" definingFeature="_p">
        <value xmi:type="uml:LiteralInteger" xmi:id="_v1" value="3"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_sl2" definingFeature="_p">
        <value xmi:type="uml:InstanceValue" xmi:id="_v2" instance="_bare"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_sl3" definingFeature="_p">
        <value xmi:type="uml:InstanceValue" xmi:id="_v3" instance="_o1"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_sl4" definingFeature="_b">
        <value xmi:type="uml:InstanceValue" xmi:id="_v4" instance="_bus1"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_sl5" definingFeature="_port">
        <value xmi:type="uml:InstanceValue" xmi:id="_v5" instance="_bus1"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_sl6" definingFeature="_u">
        <value xmi:type="uml:InstanceValue" xmi:id="_v6" instance="_m1"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_sl7" definingFeature="_pg">
        <value xmi:type="uml:InstanceValue" xmi:id="_v7" instance="_o1"/>
      </slot>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_mcs"/>
  <sysml:Block xmi:id="_s2" base_Class="_other"/>
  <sysml:InterfaceBlock xmi:id="_s3" base_Class="_if"/>
  <sysml:Block xmi:id="_s4" base_Class="_tmt"/>`)
	wantLine(t, r.Notation, "individual def 'bus 1' :> Bus;")
	wantNoLine(t, r.Notation, ":>> mcs")
	wantNote(t, r, "_sl1", migrate.Unmapped, "a part holds instances; the slot's value is a LiteralInteger")
	wantNote(t, r, "_sl2", migrate.Unmapped, "the slot's value 'snapshot' is not written as an individual: an instance specification without a classifier has no v2 form")
	wantNote(t, r, "_sl3", migrate.Unmapped, "the slot's value 'other 1' is not an instance of MCS, the type of mcs")
	wantNote(t, r, "_sl4", migrate.Unmapped, "the slot of port bus is not written: v2 has no individual port for it to be typed by")
	wantNote(t, r, "_sl5", migrate.Unmapped, "the slot of port link is not written: v2 has no individual port for it to be typed by")
	wantNote(t, r, "_sl6", migrate.Unmapped, "the slot of loose is not written: the property is written as a plain ref, which cannot be typed by an individual")
	wantNote(t, r, "_sl7", migrate.Unmapped, "the slot's value 'other 1' is an individual part def, which cannot type an item")
	wantClean(t, "bad-part-slots.sysml", r)
}

// An individual takes the kind of its classifier — `individual constraint
// def` for an instance of a constraint block — and its slots redefine the
// parameters with their `in` direction. Classifiers of another kind are not
// written: v2 does not cross them.
func TestIndividualTakesTheKindOfItsClassifier(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_fits" name="Fits">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x">`+realHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_an" name="Analysis">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_c" name="fits" type="_fits" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Rover"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_f1" name="fits 1" classifier="_fits">
      <slot xmi:type="uml:Slot" xmi:id="_sl1" definingFeature="_x">
        <value xmi:type="uml:LiteralReal" xmi:id="_v1" value="3.0"/>
      </slot>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_a1" name="analysis 1" classifier="_an">
      <slot xmi:type="uml:Slot" xmi:id="_sl2" definingFeature="_c">
        <value xmi:type="uml:InstanceValue" xmi:id="_v2" instance="_f1"/>
      </slot>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_mixed" name="mixed" classifier="_b _fits"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_bus" name="Bus"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_pf" name="port first" classifier="_bus _b"/>`, `
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_fits"/>
  <sysml:Block xmi:id="_s2" base_Class="_an"/>
  <sysml:Block xmi:id="_s3" base_Class="_b"/>
  <sysml:InterfaceBlock xmi:id="_s4" base_Class="_bus"/>`)
	wantLine(t, r.Notation, "individual constraint def 'fits 1' :> Fits {")
	wantLine(t, r.Notation, "in attribute :>> x = 3.0;")
	wantLine(t, r.Notation, "individual constraint :>> fits : 'fits 1';")
	wantLine(t, r.Notation, "individual part def mixed :> Rover;")
	wantNote(t, r, "_mixed", migrate.Approximated, "the instance's classifier Fits is not written: an individual part def cannot specialize a constraint def")
	wantLine(t, r.Notation, "individual part def 'port first' :> Rover;")
	wantNote(t, r, "_pf", migrate.Approximated, "the instance's classifier Bus is not written: an individual part def cannot specialize a port def")
	wantClean(t, "kinds.sysml", r)
}
