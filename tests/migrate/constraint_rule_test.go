package migrate_test

import (
	"testing"
)

// An owned rule is an invariant: it is written as an asserted constraint
// usage, named or unnamed, so the runtime requires it.
func TestOwnedRuleIsAsserted(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_ar" name="Valve">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ap" name="pressure">`+realHref+`</ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_cn" name="rated">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_cns">
          <body>pressure &gt; 0</body>
          <language>OCL</language>
        </specification>
      </ownedRule>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_cu">
        <specification xmi:type="uml:LiteralBoolean" xmi:id="_cus" value="true"/>
      </ownedRule>
    </packagedElement>`, `<sysml:Block xmi:id="_as" base_Class="_ar"/>`)
	wantLine(t, r.Notation, "assert constraint rated { pressure > 0 }")
	wantLine(t, r.Notation, "assert constraint { true }")
	wantClean(t, "rules.sysml", r)
}
