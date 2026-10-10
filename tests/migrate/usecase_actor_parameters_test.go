package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// actorUseCaseModel is an actor associated with a use case by an anonymous
// association that owns both ends.
const actorUseCaseModel = `
    <packagedElement xmi:type="uml:Package" xmi:id="_p" name="Use Cases">
      <packagedElement xmi:type="uml:Actor" xmi:id="_actor" name="Operator"/>
      <packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Control Instrument"/>
      <packagedElement xmi:type="uml:Association" xmi:id="_assoc" memberEnd="_eA _eU">
        <ownedEnd xmi:type="uml:Property" xmi:id="_eA" type="_actor" association="_assoc"/>
        <ownedEnd xmi:type="uml:Property" xmi:id="_eU" type="_uc" association="_assoc"/>
      </packagedElement>
    </packagedElement>`

// An actor association gives the use case an actor parameter, declared after
// its subject and subsetting the actor's one part usage, and the connection
// joins that parameter. The actor is one element: no part def, no second usage.
func TestActorAssociationIsTheUseCasesActorParameter(t *testing.T) {
	r := migrateDocument(t, actorUseCaseModel, "")
	notation := string(r.Notation)
	wantLine(t, r.Notation, "part Operator;")
	wantLine(t, r.Notation, "use case 'Control Instrument' {\n        subject;\n        actor operator :> Operator;\n        metadata MigrationMetadata::SynthesizedName about operator;\n    }")
	wantLine(t, r.Notation, "connection 'Operator to Control Instrument' connect Operator to 'Control Instrument'.operator;")
	wantNoLine(t, r.Notation, "part def Operator")
	if n := strings.Count(notation, "part Operator"); n != 1 {
		t.Errorf("part Operator written %d times, want once:\n%s", n, notation)
	}
	if n := strings.Count(notation, "actor "); n != 1 {
		t.Errorf("%d actor parameters, want one:\n%s", n, notation)
	}
	wantNote(t, r, "_eA", migrate.Approximated, "the use case declares the actor parameter operator subsetting that usage")
	wantNote(t, r, "_eU", migrate.Approximated, "the end at the use case is the connection's end at the use case's actor parameter operator")
	wantClean(t, "actor-parameter.sysml", r)
}

// A strict migration writes the same parameter and connection, marking nothing
// with the MigrationMetadata library (a made-up name is noted in a comment), and
// the output is valid under strict conformance.
func TestActorParameterInStrictMode(t *testing.T) {
	r := migrateDocumentOptions(t, actorUseCaseModel, "", migrate.Options{Strict: true})
	wantLine(t, r.Notation, "use case 'Control Instrument' {\n        subject;\n        actor operator :> Operator;\n        // names the migration made up: operator\n    }")
	wantLine(t, r.Notation, "connection 'Operator to Control Instrument' connect Operator to 'Control Instrument'.operator;")
	wantNoLine(t, r.Notation, "MigrationMetadata")
	for _, d := range errorsMode(t, "actor-parameter-strict.sysml", r.Notation, diag.ConformanceStrict) {
		t.Errorf("%v", d)
	}
}

// Two associations from one actor to one use case are two actor parameters
// with distinct names, each joined by its own connection; the actor stays one part.
func TestRepeatedActorAssociationsGetDistinctParameters(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Actor" xmi:id="_actor" name="Operator"/>
    <packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Scan"/>
    <packagedElement xmi:type="uml:Association" xmi:id="_a1" memberEnd="_e1a _e1u">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e1a" type="_actor" association="_a1"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_e1u" type="_uc" association="_a1"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_a2" memberEnd="_e2a _e2u">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2a" name="supervisor" type="_actor" association="_a2"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2u" type="_uc" association="_a2"/>
    </packagedElement>`, "")
	notation := string(r.Notation)
	wantLine(t, r.Notation, "actor operator :> Operator;")
	wantLine(t, r.Notation, "actor supervisor :> Operator;")
	wantLine(t, r.Notation, "connect Operator to Scan.operator;")
	wantLine(t, r.Notation, "connect Operator to Scan.supervisor;")
	wantNoLine(t, r.Notation, "SynthesizedName about supervisor")
	if n := strings.Count(notation, "part Operator"); n != 1 {
		t.Errorf("part Operator written %d times, want once:\n%s", n, notation)
	}
	wantClean(t, "actor-parameters.sysml", r)
}

// Two anonymous associations from one actor to one use case, neither end
// named, still get parameters that differ.
func TestUnnamedRepeatedActorAssociationsGetDistinctParameters(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Actor" xmi:id="_actor" name="Operator"/>
    <packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Scan"/>
    <packagedElement xmi:type="uml:Association" xmi:id="_a1" memberEnd="_e1a _e1u">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e1a" type="_actor" association="_a1"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_e1u" type="_uc" association="_a1"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_a2" memberEnd="_e2a _e2u">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2a" type="_actor" association="_a2"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2u" type="_uc" association="_a2"/>
    </packagedElement>`, "")
	notation := string(r.Notation)
	if n := strings.Count(notation, "actor "); n != 2 {
		t.Errorf("%d actor parameters, want two:\n%s", n, notation)
	}
	if strings.Count(notation, "actor operator :> Operator;") != 1 {
		t.Errorf("the two parameters do not differ:\n%s", notation)
	}
	wantClean(t, "actor-parameters-unnamed.sysml", r)
}

// An association whose actor end is a property the use case owns reuses that
// property as the actor parameter: no second parameter, and the connection
// joins it by its name.
func TestUseCaseOwnedActorEndIsReused(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Actor" xmi:id="_actor" name="User"/>
    <packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="customer" type="_actor" association="_assoc"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_assoc" memberEnd="_p _e2">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2" type="_uc" association="_assoc"/>
    </packagedElement>`, "")
	notation := string(r.Notation)
	wantLine(t, r.Notation, "use case Buy {\n    subject;\n    actor customer :> User;\n}")
	wantLine(t, r.Notation, "connection 'User to Buy' connect User to Buy.customer;")
	if n := strings.Count(notation, "actor "); n != 1 {
		t.Errorf("%d actor parameters, want the owned one alone:\n%s", n, notation)
	}
	if n := strings.Count(notation, "part User"); n != 1 {
		t.Errorf("part User written %d times, want once:\n%s", n, notation)
	}
	wantClean(t, "actor-owned.sysml", r)
}

// A named association is a connection def whose ends subset the actor and the
// use case, so its one connection joins the use case usage, as the def's end
// does; the use case still declares the actor parameter.
func TestNamedActorAssociationConnectsTheUseCaseUsage(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Actor" xmi:id="_actor" name="Bank"/>
    <packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Pay Fee"/>
    <packagedElement xmi:type="uml:Association" xmi:id="_assoc" name="Settlement" memberEnd="_eA _eU">
      <ownedEnd xmi:type="uml:Property" xmi:id="_eA" name="bank" type="_actor" association="_assoc"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_eU" type="_uc" association="_assoc"/>
    </packagedElement>`, "")
	wantLine(t, r.Notation, "use case 'Pay Fee' {\n    subject;\n    actor bank :> Bank;\n}")
	wantLine(t, r.Notation, "connection def Settlement {\n    end bank :> Bank;\n    end 'pay Fee' :> 'Pay Fee';")
	wantLine(t, r.Notation, "connection settlement : Settlement connect Bank to 'Pay Fee';")
	wantNoLine(t, r.Notation, "'Pay Fee'.bank")
	wantNote(t, r, "_eU", migrate.Approximated, "the end at the use case is the connection's end at the use case usage, as the connection def's end is; the use case's actor parameter bank subsets the actor")
	wantClean(t, "actor-named.sysml", r)
}

// An unnamed actor end the use case owns is named after the actor, marked as
// made up, and the connection joins it by that name.
func TestUnnamedUseCaseOwnedActorEndIsNamed(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Actor" xmi:id="_actor" name="User"/>
    <packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Buy">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" type="_actor" association="_assoc"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_assoc" memberEnd="_p _e2">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2" type="_uc" association="_assoc"/>
    </packagedElement>`, "")
	wantLine(t, r.Notation, "use case Buy {\n    subject;\n    actor user :> User;\n    metadata MigrationMetadata::SynthesizedName about user;\n}")
	wantLine(t, r.Notation, "connection 'User to Buy' connect User to Buy.user;")
	wantClean(t, "actor-owned-unnamed.sysml", r)
}

// A property of the use case typed by the actor that is not the association's
// end is not that association's parameter: the association gets its own.
func TestOwnedPropertyThatIsNotTheEndIsNotReused(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Actor" xmi:id="_actor" name="Operator"/>
    <packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Scan">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="operator" type="_actor" association="_a1"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_a1" memberEnd="_p _e1u">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e1u" type="_uc" association="_a1"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_a2" memberEnd="_e2a _e2u">
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2a" name="supervisor" type="_actor" association="_a2"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_e2u" type="_uc" association="_a2"/>
    </packagedElement>`, "")
	wantLine(t, r.Notation, "actor operator :> Operator;")
	wantLine(t, r.Notation, "actor supervisor :> Operator;")
	wantLine(t, r.Notation, "connect Operator to Scan.operator;")
	wantLine(t, r.Notation, "connect Operator to Scan.supervisor;")
	wantClean(t, "actor-owned-and-new.sysml", r)
}

// The association end's multiplicity is the actor parameter's.
func TestActorParameterKeepsTheEndsMultiplicity(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Actor" xmi:id="_actor" name="Operator"/>
    <packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Scan"/>
    <packagedElement xmi:type="uml:Association" xmi:id="_assoc" memberEnd="_eA _eU">
      <ownedEnd xmi:type="uml:Property" xmi:id="_eA" type="_actor" association="_assoc">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_lo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_hi" value="*"/>
      </ownedEnd>
      <ownedEnd xmi:type="uml:Property" xmi:id="_eU" type="_uc" association="_assoc"/>
    </packagedElement>`, "")
	wantLine(t, r.Notation, "actor operator [0..*] :> Operator;")
	wantLine(t, r.Notation, "connect Operator to Scan.operator;")
	wantClean(t, "actor-multiplicity.sysml", r)
}

// A port the use case owns, typed by the actor and the association's end, stays
// a port; the use case declares an actor parameter for the association besides.
func TestPortEndIsNotTheActorParameter(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Actor" xmi:id="_actor" name="Operator"/>
    <packagedElement xmi:type="uml:UseCase" xmi:id="_uc" name="Scan">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_p" name="console" type="_actor" association="_assoc"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_assoc" memberEnd="_p _eU">
      <ownedEnd xmi:type="uml:Property" xmi:id="_eU" type="_uc" association="_assoc"/>
    </packagedElement>`, "")
	wantLine(t, r.Notation, "actor console :> Operator;")
	wantLine(t, r.Notation, "connect Operator to Scan.console;")
	if n := strings.Count(string(r.Notation), "actor "); n != 1 {
		t.Errorf("%d actor parameters, want one:\n%s", n, r.Notation)
	}
	wantClean(t, "actor-port-end.sysml", r)
}
