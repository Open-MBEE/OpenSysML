package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// The parameter_sets fixture: an activity's {a} and {b, r} sets, an operation's
// set, an unnamed set and a set carrying a condition.
func TestParameterSets(t *testing.T) {
	r := migrateXMI(t, "parameter_sets")
	for _, line := range []string{
		"ref inputs {",
		"ref [1] = a;",
		"ref results {",
		"ref [0..*] = b;",
		"ref [1] = r;",
		"ref args {",
		"ref [1] = x;",
		"ref {",
		"ref [1] = v;",
		"ref bounded {",
		"/* not migrated: Constraint 'positive' — a parameter set's condition has no target in the transformation */",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, id := range []string{"_ps_inputs", "_ps_results", "_ps_run", "_ps_anon", "_ps_cond"} {
		wantNote(t, r, id, migrate.Mapped, "")
	}
	wantNote(t, r, "_ps_c", migrate.Unmapped, "a parameter set's condition has no target in the transformation")
	wantClean(t, "parameter_sets.sysml", r)
}

// A set binding a parameter that is not written is approximated; one binding
// none is unmapped.
func TestParameterSetOfAnUnwrittenParameter(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Activity" xmi:id="_act" name="Collect">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_pk" name="k" direction="in">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedParameter>
      <ownedParameterSet xmi:type="uml:ParameterSet" xmi:id="_set1" name="mixed" parameter="_pk _gone"/>
      <ownedParameterSet xmi:type="uml:ParameterSet" xmi:id="_set2" name="empty" parameter="_gone"/>
    </packagedElement>`, "")
	out := string(r.Notation)
	if strings.Contains(out, "gone") {
		t.Errorf("a parameter that is not in the document is bound by a set:\n%s", out)
	}
	wantLine(t, r.Notation, "ref mixed {")
	wantLine(t, r.Notation, "ref [1] = k;")
	wantNote(t, r, "_set1", migrate.Approximated, "not in the document and are not written")
	wantNote(t, r, "_set2", migrate.Unmapped, "the set binds no parameter that is written")
}

// A set on an operation's method binds the feature the operation declares: a
// realized parameter is referenced by the operation parameter's name, not by a
// name the method declared.
func TestParameterSetBindsTheOperationParameterAMethodParameterRealizes(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C">
      <ownedOperation xmi:type="uml:Operation" xmi:id="_op" name="Run" method="_body">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_x" name="x" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_body" name="Body" specification="_op">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_y" name="y" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameterSet xmi:type="uml:ParameterSet" xmi:id="_set" name="args" parameter="_y"/>
      </ownedBehavior>
    </packagedElement>`, "")
	wantLine(t, r.Notation, "ref [1] = x;")
	out := string(r.Notation)
	if strings.Contains(out, "= y;") {
		t.Errorf("the set binds the method parameter's name, which is not declared:\n%s", out)
	}
	wantNote(t, r, "_set", migrate.Mapped, "")
	wantClean(t, "parameter_set_realized.sysml", r)
}

// A method parameter shadowed by the operation's own parameter is left out of
// the set: its name resolves to a feature that differs in direction or type.
func TestParameterSetLeavesOutAShadowedParameter(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_c" name="C">
      <ownedOperation xmi:type="uml:Operation" xmi:id="_op" name="Run" method="_body">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_x" name="x" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_body" name="Body" specification="_op">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_mx" name="x" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#String"/>
        </ownedParameter>
        <ownedParameterSet xmi:type="uml:ParameterSet" xmi:id="_set" name="args" parameter="_mx"/>
      </ownedBehavior>
    </packagedElement>`, "")
	wantNote(t, r, "_set", migrate.Unmapped, "shares its name with the operation's parameter and is not written separately")
	wantClean(t, "parameter_set_shadowed.sysml", r)
}
