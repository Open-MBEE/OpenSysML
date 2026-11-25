package migrate_test

import (
	"bytes"
	"testing"
)

const constraintInheritedParameterKindsModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_vehicle" name="Vehicle"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_tick" name="Tick"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_base" name="Base">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bv" name="vehicle" type="_vehicle"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bt" name="tick" type="_tick"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bl" name="limit">` + realHref + `</ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_brule" constrainedElement="_base">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_bspec">
          <body>limit &gt; 0</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_child" name="Child">
      <generalization xmi:type="uml:Generalization" xmi:id="_g" general="_base"/>
    </packagedElement>
`

const constraintInheritedParameterKindsApplications = `
  <sysml:Block xmi:id="_s0" base_Class="_vehicle"/>
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_base"/>
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_child"/>
`

// A restated inherited parameter keeps the kind its general wrote it with: a block-typed
// one is a reference part, a signal-typed one a reference item, a value an attribute.
func TestRestatedInheritedParameterKeepsItsKind(t *testing.T) {
	r := migrateDocument(t, constraintInheritedParameterKindsModel, constraintInheritedParameterKindsApplications)
	wantLine(t, r.Notation, "in ref part vehicle : Vehicle[1];")
	wantLine(t, r.Notation, "in ref part :>> vehicle;")
	wantLine(t, r.Notation, "in ref item :>> tick;")
	wantLine(t, r.Notation, "in attribute :>> limit;")
	wantNoLine(t, r.Notation, "in attribute :>> vehicle;")
	wantNoLine(t, r.Notation, "in attribute :>> tick;")
	session(t, r)
}

const constraintInheritedPortParameterModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_base" name="Base">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bl" name="limit">` + realHref + `</ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bm" name="margin">` + realHref + `</ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_brule" constrainedElement="_base">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_bspec">
          <body>margin &lt;= limit</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_child" name="Child">
      <generalization xmi:type="uml:Generalization" xmi:id="_g" general="_base"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_cl" name="limit" aggregation="composite"/>
    </packagedElement>
`

const constraintInheritedPortParameterApplications = `
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_base"/>
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_child"/>
`

// A port a derived constraint block owns under a general's parameter name is that
// parameter's restatement, written once.
func TestDerivedConstraintBlockPortIsTheInheritedParameter(t *testing.T) {
	r := migrateDocument(t, constraintInheritedPortParameterModel, constraintInheritedPortParameterApplications)
	wantLine(t, r.Notation, "in attribute :>> margin;")
	wantLine(t, r.Notation, "in attribute limit[1] :>> limit;")
	wantNoLine(t, r.Notation, "in attribute :>> limit;")
	if n := bytes.Count(r.Notation, []byte(":>> limit")); n != 1 {
		t.Fatalf("Child must redefine limit once, got %d redefinitions:\n%s", n, r.Notation)
	}
	session(t, r)
}

// A port the migrator skips as tool content writes nothing, so the inherited
// parameter under its name keeps its position through the restatement.
const constraintToolContentPortModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_base" name="Base">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bl" name="limit">` + realHref + `</ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bm" name="margin">` + realHref + `</ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_brule" constrainedElement="_base">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_bspec">
          <body>margin &lt;= limit</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_child" name="Child">
      <generalization xmi:type="uml:Generalization" xmi:id="_g" general="_base"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_cp" name="limit"/>
    </packagedElement>
`

func TestSkippedToolContentPortLeavesTheInheritedParameterToTheRestatement(t *testing.T) {
	r := migrateDocument(t, constraintToolContentPortModel, `
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_base"/>
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_child"/>
  <ui:Button xmlns:ui="http://www.magicdraw.com/schemas/UI_Prototyping_Profile.xmi" xmi:id="_s3" base_Port="_cp"/>`)
	wantLine(t, r.Notation, "in attribute :>> limit;")
	wantLine(t, r.Notation, "in attribute :>> margin;")
	wantNoLine(t, r.Notation, "in attribute limit[1] :>> limit;")
	if n := bytes.Count(r.Notation, []byte(":>> limit")); n != 1 {
		t.Fatalf("Child must redefine limit once, got %d redefinitions:\n%s", n, r.Notation)
	}
	session(t, r)
}
