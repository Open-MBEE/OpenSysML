package migrate_test

import (
	"bytes"
	"testing"
)

const constraintInheritedParametersModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_base" name="Base">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bl" name="inLimit">` + realHref + `</ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bm" name="margin">` + realHref + `</ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_brule" constrainedElement="_base">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_bspec">
          <body>margin &lt;= inLimit</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_child" name="Child">
      <generalization xmi:type="uml:Generalization" xmi:id="_g" general="_base"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_cl" name="inLimit">` + realHref + `</ownedAttribute>
    </packagedElement>
`

const constraintInheritedParametersApplications = `
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_base"/>
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_child"/>
`

// A derived constraint block restates each general's parameter by clause before its own, so
// none is redefined by position; a parameter it owns under a general's name is that
// restatement and is written once.
func TestDerivedConstraintBlockRestatesInheritedParametersOnce(t *testing.T) {
	r := migrateDocument(t, constraintInheritedParametersModel, constraintInheritedParametersApplications)
	wantLine(t, r.Notation, "in attribute :>> margin;")
	wantNoLine(t, r.Notation, "in attribute :>> inLimit;")
	if n := bytes.Count(r.Notation, []byte(":>> inLimit")); n != 1 {
		t.Fatalf("Child must declare inLimit once, got %d declarations:\n%s", n, r.Notation)
	}
	session(t, r)
}
