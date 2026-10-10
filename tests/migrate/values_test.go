package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
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
// classified by both a block and a value type is the part alone.
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
	wantLine(t, r.Notation, "part mixed : Rover {")
	wantNoLine(t, r.Notation, "x = 1.0")
	wantNote(t, r, "_home", migrate.Mapped, "")
	wantNote(t, r, "_mixed", migrate.Approximated, "the instance's classifier Position is not written: a part cannot be typed by an attribute def")
	wantNote(t, r, "_ms", migrate.Unmapped, "the slot's defining feature Position::x is not a feature of any classifier the instance is written to be typed by")
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
      <ownedAttribute xmi:type="uml:Property" xmi:id="_serial" name="serial">`+integerHref+`
        <defaultValue xmi:type="uml:LiteralString" xmi:id="_sv" value="9223372036854775808"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_span" name="span">`+realHref+`
        <defaultValue xmi:type="uml:LiteralString" xmi:id="_spv" value="1e400"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_b"/>`)
	wantLine(t, r.Notation, "attribute gain : ScalarValues::Real default = 17.0;")
	wantLine(t, r.Notation, "attribute poles : ScalarValues::Integer default = 4;")
	wantLine(t, r.Notation, "attribute on : ScalarValues::Boolean default = true;")
	wantLine(t, r.Notation, "attribute serial : ScalarValues::Integer default = 9223372036854775808;")
	wantLine(t, r.Notation, "attribute span : ScalarValues::Real default = 1e400;")
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

// A constraint yields a Boolean, so a specification that is a literal or
// instance of anything else is left as a comment, whether the constraint is
// a block's own rule or a constraint block's body; a Boolean literal, a
// string spelling one, a Boolean expression and an OCL body that is one are written.
func TestNonBooleanConstraintSpecificationIsNotWritten(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Enumeration" xmi:id="_mode" name="Mode">
      <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_on" name="on"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Lamp">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_lit" name="lit">`+booleanHref+`</ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_c1" name="one">
        <specification xmi:type="uml:LiteralInteger" xmi:id="_c1s" value="1"/>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_c2" name="half">
        <specification xmi:type="uml:LiteralReal" xmi:id="_c2s" value="0.5"/>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_c3" name="word">
        <specification xmi:type="uml:LiteralString" xmi:id="_c3s" value="ok"/>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_c4" name="mode">
        <specification xmi:type="uml:InstanceValue" xmi:id="_c4s" instance="_on"/>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_c5" name="two">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_c5s">
          <language>OCL</language>
          <body>2</body>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_c6" name="always">
        <specification xmi:type="uml:LiteralBoolean" xmi:id="_c6s" value="true"/>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_c7" name="spelled">
        <specification xmi:type="uml:LiteralString" xmi:id="_c7s" value="false"/>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_c8" name="lighted">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_c8s">
          <language>OCL</language>
          <body>lit or not lit</body>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_c9" name="yes">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_c9s">
          <language>OCL</language>
          <body>true</body>
        </specification>
      </ownedRule>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_cb" name="Rule">
      <ownedRule xmi:type="uml:Constraint" xmi:id="_r">
        <specification xmi:type="uml:LiteralInteger" xmi:id="_rs" value="3"/>
      </ownedRule>
    </packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_b"/><sysml:ConstraintBlock xmi:id="_scb" base_Class="_cb"/>`)
	wantLine(t, r.Notation, "constraint always { true }")
	wantLine(t, r.Notation, "constraint spelled { false }")
	wantLine(t, r.Notation, "constraint lighted { lit or not lit }")
	wantLine(t, r.Notation, "constraint yes { true }")
	wantNoLine(t, r.Notation, "constraint one {")
	wantNoLine(t, r.Notation, "constraint half {")
	wantNoLine(t, r.Notation, "constraint word {")
	wantNoLine(t, r.Notation, "constraint mode {")
	wantNoLine(t, r.Notation, "constraint two {")
	wantNoLine(t, r.Notation, "    3\n")
	wantNote(t, r, "_c1", migrate.Unmapped, "the integer 1 is not a value of Boolean, which the constraint yields")
	wantNote(t, r, "_c2", migrate.Unmapped, "the real 0.5 is not a value of Boolean, which the constraint yields")
	wantNote(t, r, "_c3", migrate.Unmapped, `the string "ok" is not a value of Boolean, which the constraint yields`)
	wantNote(t, r, "_c4", migrate.Unmapped, "the literal Mode::on is not a value of Boolean, which the constraint yields")
	wantNote(t, r, "_c5", migrate.Unmapped, "the integer 2 is not a value of Boolean, which the constraint yields")
	wantNote(t, r, "_c6", migrate.Mapped, "")
	wantNote(t, r, "_c7", migrate.Approximated, `the string "false" is written as the Boolean the constraint yields`)
	wantNote(t, r, "_c8", migrate.Approximated, "opaque expression copied verbatim (language OCL)")
	wantNote(t, r, "_r", migrate.Unmapped, "the integer 3 is not a value of Boolean, which the constraint yields")
	wantClean(t, "constraints.sysml", r)
}

// A value action's literal, opaque or not, is checked against its result pin's
// type as a default is against its feature's: a string spelling no number is
// not a Real result, a numeric string is written as the Real it spells.
func TestValueActionLiteralTakesTheResultPinsType(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Stage">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_deg" name="deg">
          <value xmi:type="uml:LiteralString" xmi:id="_degV" value="-1deg"/>
          <result xmi:type="uml:OutputPin" xmi:id="_degOut" name="result">`+realHref+`</result>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_num" name="num">
          <value xmi:type="uml:LiteralString" xmi:id="_numV" value="-1"/>
          <result xmi:type="uml:OutputPin" xmi:id="_numOut" name="result">`+realHref+`</result>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_script" name="script">
          <value xmi:type="uml:OpaqueExpression" xmi:id="_scriptV">
            <language>JavaScript</language>
            <body>"2deg"</body>
          </value>
          <result xmi:type="uml:OutputPin" xmi:id="_scriptOut" name="result">`+realHref+`</result>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_deg"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_deg" target="_num"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_num" target="_script"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_script" target="_final"/>
      </ownedBehavior>
    </packagedElement>`, `<sysml:Block xmi:id="_sb" base_Class="_b"/>`)
	wantLine(t, r.Notation, "out result : ScalarValues::Real[1] = -1.0;")
	wantNoLine(t, r.Notation, `= "-1deg";`)
	wantNoLine(t, r.Notation, `= "2deg";`)
	wantNote(t, r, "_deg", migrate.Approximated, `the value -1deg is not written: the string "-1deg" is not a value of Real, which the feature holds`)
	wantNote(t, r, "_num", migrate.Approximated, `the string "-1" is written as the Real the feature holds`)
	wantNote(t, r, "_script", migrate.Approximated, `the value {JavaScript} "2deg" is not written: the types at "\"2deg\"" disagree: the expression is a String, not the Real wanted`)
	wantClean(t, "value_pins.sysml", r)
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

// A slot whose values contradict its feature — too few or too many for the
// multiplicity, none for a required feature, or a repeat on a unique feature —
// is invalid in both languages and is left as a comment naming the values.
// An empty slot of an optional feature is a redefinition bound to nothing.
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
      <ownedAttribute xmi:type="uml:Property" xmi:id="_need" name="need">`+realHref+`</ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_may" name="may">`+realHref+`
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_ml"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_mu" value="1"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i" name="cal" classifier="_cal">
      <slot xmi:id="_st" definingFeature="_t">
        <value xmi:type="uml:LiteralReal" xmi:id="_v1" value="815.0"/>
        <value xmi:type="uml:LiteralReal" xmi:id="_v2" value="815.0"/>
      </slot>
      <slot xmi:id="_sn" definingFeature="_need"/>
      <slot xmi:id="_sm" definingFeature="_may"/>
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
	wantNoLine(t, r.Notation, "attribute :>> need")
	wantLine(t, r.Notation, "attribute :>> may;")
	wantNote(t, r, "_st", migrate.Unmapped, "the slot repeats the value 815.0 on a unique feature; its values are 815.0, 815.0")
	wantNote(t, r, "_sp", migrate.Unmapped, "the slot holds 1 value(s) for a feature of multiplicity 3")
	wantNote(t, r, "_sn", migrate.Unmapped, "the slot holds 0 value(s) for a feature of multiplicity 1")
	wantNote(t, r, "_sm", migrate.Mapped, "")
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
	wantNote(t, r, "_s", migrate.Unmapped, "the slot's defining feature B::y is not a feature of any classifier the instance is written to be typed by")
	wantClean(t, "foreign.sysml", r)
}

// A default that names an instance is written as the usage's default value:
// the instance is a usage, which an expression can name.
func TestInstanceDefaultIsTheUsageDefault(t *testing.T) {
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
	wantLine(t, r.Notation, "part usual : Table;")
	wantLine(t, r.Notation, "part angles : Table[0..*] default = usual;")
	wantNote(t, r, "_p", migrate.Mapped, "")
	wantClean(t, "instance-default.sysml", r)
}

// An untyped property whose default is an instance keeps the default and
// stays untyped: the instance is a usage, not a type.
func TestUntypedPropertyKeepsItsInstanceDefault(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_tab" name="Table"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_std" name="usual" classifier="_tab"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Procedure">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="angles">
        <defaultValue xmi:type="uml:InstanceValue" xmi:id="_dv" instance="_std"/>
      </ownedAttribute>
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_tab"/><sysml:Block xmi:id="_s2" base_Class="_b"/>`)
	wantLine(t, r.Notation, "ref angles default = usual;")
	wantNoLine(t, r.Notation, "default value not migrated")
	wantNote(t, r, "_p", migrate.Mapped, "")
	wantClean(t, "untyped-instance-default.sysml", r)
}

// An instance is a usage's default only when it is an instance of the usage's
// type: a port's default may be an instance of its interface block, and an
// instance of an unrelated block is no instance of the type, so such a default
// stays a comment.
func TestInstanceDefaultOfAnotherTypeIsNotADefault(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_bus" name="Bus"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_fits" name="Fits"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_b1" name="bus 1" classifier="_bus"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_f1" name="fits 1" classifier="_fits"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_p1" name="pump 1" classifier="_pump"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_v" name="Vehicle">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_e" name="engine" type="_engine" aggregation="composite">
        <defaultValue xmi:type="uml:InstanceValue" xmi:id="_de" instance="_p1"/>
      </ownedAttribute>
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
  <sysml:Block xmi:id="_s3" base_Class="_v"/>
  <sysml:Block xmi:id="_s4" base_Class="_engine"/>
  <sysml:Block xmi:id="_s5" base_Class="_pump"/>`)
	wantLine(t, r.Notation, "port 'bus 1' : Bus;")
	wantLine(t, r.Notation, "port bus : Bus default = 'bus 1';")
	wantLine(t, r.Notation, "part engine : Engine {")
	wantNoLine(t, r.Notation, "default = 'pump 1'")
	wantLine(t, r.Notation, "part check : Fits default = 'fits 1';")
	wantLine(t, r.Notation, "ref loose default = 'bus 1';")
	wantNote(t, r, "_p", migrate.Mapped, "")
	wantNote(t, r, "_e", migrate.Approximated, "default value not migrated: the default value 'pump 1' is not an instance of Engine, the type of engine")
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
      <ownedAttribute xmi:type="uml:Property" xmi:id="_m_prop" name="m" type="_mass">
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
	wantLine(t, r.Notation, "in item fuel : Fuel[1];")
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
	wantLine(t, r.Notation, "binding feed bind tank = engine.intake;")
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
	wantLine(t, r.Notation, "part :>> mcs = 'mcs 1';")
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

// A slot of a part property holds instances: the redefinition of the part is
// bound to the instance usage, or to the sequence of several. A reference
// part keeps its `ref`.
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
	wantLine(t, r.Notation, "part 'mcs 1' : FastMCS;")
	wantLine(t, r.Notation, "part 'tmt 1' : TMT {")
	wantLine(t, r.Notation, "part :>> mcs = 'mcs 1';")
	wantLine(t, r.Notation, "part :>> spares = ('mcs 1', 'mcs 2');")
	wantLine(t, r.Notation, "ref part :>> shared = 'mcs 2';")
	wantLine(t, r.Notation, "part :>> spares = 'mcs 2';")
	for _, id := range []string{"_sl1", "_sl2", "_sl3", "_sl4"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("entries for %s = %+v", id, es)
		}
	}
	wantClean(t, "part-slots.sysml", r)
}

// A composite part written unique, although v1 declares it nonunique, cannot
// hold a repeated instance: the slot is unmapped as on any unique feature.
func TestPartSlotRepeatingOnAForcedUniquePartIsUnmapped(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_mcs" name="MCS"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_tmt" name="TMT">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_q" name="spares" type="_mcs" aggregation="composite" isUnique="false">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_lo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_up" value="*"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_m2" name="mcs 2" classifier="_mcs"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_t1" name="tmt 1" classifier="_tmt">
      <slot xmi:type="uml:Slot" xmi:id="_sl" definingFeature="_q">
        <value xmi:type="uml:InstanceValue" xmi:id="_v1" instance="_m2"/>
        <value xmi:type="uml:InstanceValue" xmi:id="_v2" instance="_m2"/>
      </slot>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_mcs"/>
  <sysml:Block xmi:id="_s3" base_Class="_tmt"/>`)
	wantLine(t, r.Notation, "part spares : MCS[0..*];")
	wantNote(t, r, "_q", migrate.Approximated, "nonunique is not written: a part in a part def implicitly subsets Items::Item::subparts, which is unique")
	wantNote(t, r, "_sl", migrate.Unmapped, "the slot repeats the value 'mcs 2' on a unique feature; its values are 'mcs 2', 'mcs 2'")
	wantClean(t, "part-slot-repeat.sysml", r)
}

// A part slot whose value is not an instance of the part's type has no v2
// form: a literal, an instance without a classifier, an instance of another
// block or of a block where an item is due, or a part where an attribute is
// due; a port's slot is bound to the instance of its interface block.
func TestPartSlotWithoutAConformingInstanceIsUnmapped(t *testing.T) {
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
	wantLine(t, r.Notation, "port 'bus 1' : Bus;")
	wantNoLine(t, r.Notation, ":>> mcs")
	wantNote(t, r, "_sl1", migrate.Unmapped, "a part holds instances; the slot's value is a LiteralInteger")
	wantNote(t, r, "_sl2", migrate.Unmapped, "the slot's value 'snapshot' is not written as a usage: an instance specification without a classifier has no v2 form")
	wantNote(t, r, "_sl3", migrate.Unmapped, "the slot's value 'other 1' is not an instance of MCS, the type of mcs")
	wantLine(t, r.Notation, "port :>> bus = 'bus 1';")
	wantLine(t, r.Notation, "port :>> link = 'bus 1';")
	wantNote(t, r, "_sl4", migrate.Mapped, "")
	wantNote(t, r, "_sl5", migrate.Mapped, "")
	wantNoLine(t, r.Notation, ":>> loose")
	wantNote(t, r, "_sl6", migrate.Unmapped, "the slot's value 'mcs 1' is written as a part, which cannot be the value of an attribute")
	wantNote(t, r, "_sl7", migrate.Unmapped, "the slot's value 'other 1' is not an instance of Ping, the type of ping")
	wantClean(t, "bad-part-slots.sysml", r)
}

// A slot of an untyped part — composite, or marked «PartProperty» — holds any
// part instance: the property has no type to check the instance against.
func TestUntypedPartSlotHoldsAnyInstance(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_wheel" name="Wheel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_w" name="wheel" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_s" name="spare"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_w1" name="wheel 1" classifier="_wheel"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_c1" name="car 1" classifier="_car">
      <slot xmi:type="uml:Slot" xmi:id="_sw" definingFeature="_w">
        <value xmi:type="uml:InstanceValue" xmi:id="_vw" instance="_w1"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_ss" definingFeature="_s">
        <value xmi:type="uml:InstanceValue" xmi:id="_vs" instance="_w1"/>
      </slot>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_wheel"/>
  <sysml:Block xmi:id="_s2" base_Class="_car"/>
  <MD_Customization_for_SysML__additional_stereotypes:PartProperty xmlns:MD_Customization_for_SysML__additional_stereotypes="http://www.magicdraw.com/spec/Customization/180/SysML" xmi:id="_mk" base_Property="_s"/>`)
	wantLine(t, r.Notation, "part wheel;")
	wantLine(t, r.Notation, "part spare;")
	wantLine(t, r.Notation, "part :>> wheel = 'wheel 1';")
	wantLine(t, r.Notation, "part :>> spare = 'wheel 1';")
	wantNote(t, r, "_sw", migrate.Mapped, "")
	wantNote(t, r, "_ss", migrate.Mapped, "")
	wantClean(t, "untyped-part-slots.sysml", r)
}

// An untyped part that redefines a typed one, by declaration or by sharing its
// name, inherits the type: a slot holding an instance of another type is unmapped.
func TestUntypedRedefiningPartSlotKeepsTheInheritedType(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_wheel" name="Wheel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_base" name="Base">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bw" name="wheel" type="_wheel" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bs" name="spare" type="_wheel" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <generalization xmi:type="uml:Generalization" xmi:id="_g" general="_base"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_w" name="wheel" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_s" name="front" aggregation="composite" redefinedProperty="_bs"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_w1" name="wheel 1" classifier="_wheel"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_e1" name="engine 1" classifier="_engine"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_c1" name="car 1" classifier="_car">
      <slot xmi:type="uml:Slot" xmi:id="_sw" definingFeature="_w">
        <value xmi:type="uml:InstanceValue" xmi:id="_vw" instance="_e1"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_ss" definingFeature="_s">
        <value xmi:type="uml:InstanceValue" xmi:id="_vs" instance="_w1"/>
      </slot>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_wheel"/>
  <sysml:Block xmi:id="_s2" base_Class="_engine"/>
  <sysml:Block xmi:id="_s3" base_Class="_base"/>
  <sysml:Block xmi:id="_s4" base_Class="_car"/>`)
	wantLine(t, r.Notation, "part wheel :>> wheel;")
	wantLine(t, r.Notation, "part front :>> spare;")
	wantNoLine(t, r.Notation, ":>> wheel = 'engine 1'")
	wantNote(t, r, "_sw", migrate.Unmapped, "the slot's value 'engine 1' is not an instance of Wheel, the type of wheel")
	wantLine(t, r.Notation, "part :>> front = 'wheel 1';")
	wantNote(t, r, "_ss", migrate.Mapped, "")
	wantClean(t, "redefining-part-slots.sysml", r)
}

// An untyped part subsetting several typed ones must hold an instance of every
// type: an instance of only the first is unmapped, one of all is written.
func TestUntypedPartSlotKeepsEveryInheritedType(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_vehicle" name="Vehicle"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_wheel" name="Wheel">
      <generalization xmi:type="uml:Generalization" xmi:id="_g" general="_vehicle"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_v" name="vehicle" type="_vehicle" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_w" name="wheel" type="_wheel" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_s" name="spare" aggregation="composite" subsettedProperty="_v _w"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_t" name="trailer" aggregation="composite" subsettedProperty="_v _w"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_v1" name="vehicle 1" classifier="_vehicle"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_w1" name="wheel 1" classifier="_wheel"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_c1" name="car 1" classifier="_car">
      <slot xmi:type="uml:Slot" xmi:id="_ss" definingFeature="_s">
        <value xmi:type="uml:InstanceValue" xmi:id="_vs" instance="_v1"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_st" definingFeature="_t">
        <value xmi:type="uml:InstanceValue" xmi:id="_vt" instance="_w1"/>
      </slot>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_vehicle"/>
  <sysml:Block xmi:id="_s2" base_Class="_wheel"/>
  <sysml:Block xmi:id="_s3" base_Class="_car"/>`)
	wantLine(t, r.Notation, "part spare :> vehicle :> wheel;")
	wantNoLine(t, r.Notation, ":>> spare = 'vehicle 1'")
	wantNote(t, r, "_ss", migrate.Unmapped, "the slot's value 'vehicle 1' is not an instance of Wheel, the type of spare")
	wantLine(t, r.Notation, "part :>> trailer = 'wheel 1';")
	wantNote(t, r, "_st", migrate.Mapped, "")
	wantClean(t, "subsetting-part-slots.sysml", r)
}

// An instance takes the kind of its classifier — a `constraint` usage for an
// instance of a constraint block — and its slots redefine the parameters with
// their `in` direction. Classifiers of another kind are not
// written: v2 does not cross them.
func TestInstanceTakesTheKindOfItsClassifier(t *testing.T) {
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
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_mixed" name="mixed" classifier="_b _fits">
      <slot xmi:type="uml:Slot" xmi:id="_sl3" definingFeature="_x">
        <value xmi:type="uml:LiteralReal" xmi:id="_v3" value="4.0"/>
      </slot>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_bus" name="Bus"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_pf" name="port first" classifier="_bus _b"/>`, `
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_fits"/>
  <sysml:Block xmi:id="_s2" base_Class="_an"/>
  <sysml:Block xmi:id="_s3" base_Class="_b"/>
  <sysml:InterfaceBlock xmi:id="_s4" base_Class="_bus"/>`)
	wantLine(t, r.Notation, "constraint 'fits 1' : Fits {")
	wantLine(t, r.Notation, "in attribute :>> x = 3.0;")
	wantLine(t, r.Notation, "constraint :>> fits = 'fits 1';")
	wantLine(t, r.Notation, "part mixed : Rover {")
	wantNote(t, r, "_mixed", migrate.Approximated, "the instance's classifier Fits is not written: a part cannot be typed by a constraint def")
	wantNoLine(t, r.Notation, "attribute :>> x = 4.0;")
	wantNote(t, r, "_sl3", migrate.Unmapped, "the slot's defining feature Fits::x is not a feature of any classifier the instance is written to be typed by")
	wantLine(t, r.Notation, "port 'port first' : Bus;")
	wantNote(t, r, "_pf", migrate.Approximated, "the instance's classifier Rover is not written: a port cannot be typed by a part def")
	wantClean(t, "kinds.sysml", r)
}

// A simulation tool records its verdict on a constraint property in the
// result instance's slot for it, as a literal of a verdict enumeration outside
// the document; a usage has no slot for a verdict, and the note says
// that is what the slot holds rather than that the literal is out of reach.
func TestConstraintSlotHoldingAVerdictIsUnmappedAsOne(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_fits" name="Fits">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x">`+realHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_an" name="Analysis">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_c" name="fits" type="_fits" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_a1" name="analysis 1" classifier="_an">
      <slot xmi:type="uml:Slot" xmi:id="_sl1" definingFeature="_c">
        <value xmi:type="uml:InstanceValue" xmi:id="_v1">
          <instance href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML_dataType.VerdictKind.pass">
            <xmi:Extension extender="MagicDraw UML 2024x">
              <referenceExtension referentPath="SysML::Requirements::VerdictKind::pass" referentType="EnumerationLiteral"/>
            </xmi:Extension>
          </instance>
        </value>
      </slot>
    </packagedElement>`, `
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_fits"/>
  <sysml:Block xmi:id="_s2" base_Class="_an"/>`)
	wantLine(t, r.Notation, "part 'analysis 1' : Analysis {")
	wantNoLine(t, r.Notation, ":>> fits")
	wantNote(t, r, "_sl1", migrate.Unmapped, "the slot of constraint fits holds the literal SysML::Requirements::VerdictKind::pass, the run's verdict on the constraint rather than an instance of its type; a usage has no slot for a verdict")
	wantClean(t, "verdict-slot.sysml", r)
}

// An instance classified by an actor, a part usage once migrated, is a part
// usage subsetting it: one of the actor's instances.
func TestInstanceOfActorSubsetsTheUsage(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Actor" xmi:id="_d" name="Driver"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_alice" name="alice" classifier="_d"/>`, ``)
	wantLine(t, r.Notation, "part Driver;")
	wantLine(t, r.Notation, "part alice :> Driver;")
	wantNote(t, r, "_alice", migrate.Mapped, "")
	wantClean(t, "actor-instance.sysml", r)
}
