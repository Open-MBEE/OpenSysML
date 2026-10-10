package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// An owned rule is an invariant, written as the OMG mapping does: a constraint
// def whose result is the specification, nested in the constrained block, and an
// asserted usage typed by it, so the runtime requires it. The def reads the
// block's features through its context parameter, which the usage binds to the
// block. A named rule names the usage; the def takes the capitalized name. An
// unnamed rule leaves the usage unnamed and the def takes a fresh name.
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
    </packagedElement>`,
		`<sysml:Block xmi:id="_as" base_Class="_ar"/>`)
	wantBlock(t, r.Notation,
		"part def Valve {",
		"attribute pressure : ScalarValues::Real;",
		"constraint def Rated {",
		"in ref context : Valve[1];",
		"context.pressure > 0",
		"}",
		"assert constraint rated : Rated { in ref :>> context = this; }",
		"constraint def Constraint {",
		"true",
		"}",
		"assert constraint : Constraint;",
		"}")
	wantNoLine(t, r.Notation, "assert constraint rated {")
	wantNote(t, r, "_cn", migrate.Approximated, "the constraint def Rated holds the specification as its result, asserted by the usage typed by it; opaque expression copied verbatim (language OCL)")
	wantNote(t, r, "_cu", migrate.Mapped, "the constraint def Constraint holds the specification as its result, asserted by the usage typed by it")
	wantClean(t, "rules.sysml", r)
}

// A rule named like its block's def spelling, or like a sibling, takes a fresh
// def name, and the usage keeps the rule's name.
func TestOwnedRuleDefNameAvoidsItsSiblings(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_ar" name="Valve">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ap" name="Rated">`+booleanHref+`</ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_cn" name="rated">
        <specification xmi:type="uml:LiteralBoolean" xmi:id="_cns" value="true"/>
      </ownedRule>
    </packagedElement>`,
		`<sysml:Block xmi:id="_as" base_Class="_ar"/>`)
	wantLine(t, r.Notation, "attribute Rated : ScalarValues::Boolean;")
	wantLine(t, r.Notation, "constraint def 'Rated 2' {")
	wantLine(t, r.Notation, "assert constraint rated : 'Rated 2';")
	wantClean(t, "rules.sysml", r)
}
