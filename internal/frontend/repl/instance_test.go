package repl

import (
	"strings"
	"testing"
)

// TestInstantiatedModelEvaluatesDerivedFeatureValues is the "executable model" contract:
// after %instantiate, an attribute defined in terms of other features reports a
// value rather than <unknown>, including through a nested part.
func TestInstantiatedModelEvaluatesDerivedFeatureValues(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")

	got := run(t, s, "%features Derived::Vehicle")
	wants(t, got,
		"mass = 1500.0",    // declared constant
		"doubled = 3000.0", // derived from a sibling feature
		"total = 1770.0",   // derived through a nested part: 1500 + 300*0.9
	)
	rejects(t, got, "<unknown>")
}

// A derived value is reachable by %eval too, and reports which instance
// produced it.
func TestEvalReadsDerivedFeatureValueOfInstance(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")

	wants(t, run(t, s, "%eval Derived::Vehicle::doubled"),
		"✓ Derived::Vehicle::doubled (on Derived::Vehicle ID: 1)",
		"= 3000.0",
	)
}

// Without an instance the same name still evaluates, using declared defaults,
// and says nothing about an instance.
func TestEvalWithoutInstanceUsesDeclaredDefault(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	got := run(t, s, "%eval Derived::Vehicle::mass")
	wants(t, got, "= 1500.0")
	rejects(t, got, "(on ")
}

// TestConstraintBindsToInstance: the same constraint text gives opposite
// verdicts on two instances, which is only possible if evaluation sees the
// instance's feature values.
func TestConstraintBindsToInstance(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")
	run(t, s, "%instantiate Derived::Heavy")

	wants(t, run(t, s, "%constraint Derived::Vehicle::massOK"),
		"✓ Constraint Derived::Vehicle::massOK passed (on Derived::Vehicle ID: 1)")

	failed := run(t, s, "%constraint Derived::Heavy::massOK")
	wants(t, failed,
		"✗ Constraint Derived::Heavy::massOK failed (on Derived::Heavy ID:",
		"Assertion evaluated to false",
	)
	// A false assertion is the model's answer, not a malfunction.
	rejects(t, failed, "Error:")
}

// %features renders a constraint feature as a verdict; a feature value would be
// meaningless for it.
func TestFeatureValuesRendersConstraintVerdict(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")
	run(t, s, "%instantiate Derived::Heavy")

	wants(t, run(t, s, "%features Derived::Vehicle"), "massOK: <constraint: satisfied>")
	wants(t, run(t, s, "%features Derived::Heavy"), "massOK: <constraint: violated>")
}

// A requirement usage is a verdict too, and %features must agree with what
// %requirement says about the same feature of the same instance.
func TestFeatureValuesAgreesWithRequirementOnSameInstance(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Vehicle")

	wants(t, run(t, s, "%requirement Derived::Vehicle::lightEnough"), "satisfied")
	got := run(t, s, "%features Derived::Vehicle")
	wants(t, got, "lightEnough: <requirement: satisfied>")
	rejects(t, got, "<unknown>")
}

// A required condition that is false is the model's answer, not a malfunction,
// so it reports a verdict rather than an error — in both places that report it.
func TestRequirementViolationIsAVerdictNotAnError(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	run(t, s, "%instantiate Derived::Heavy")

	got := run(t, s, "%requirement Derived::Heavy::lightEnough")
	wants(t, got, "✗ Requirement Derived::Heavy::lightEnough failed", "Required condition evaluated to false")
	rejects(t, got, "Error:")
	wants(t, run(t, s, "%features Derived::Heavy"), "lightEnough: <requirement: violated>")
}

// A constraint over a feature nothing declares is still an error, distinct from
// a violated assertion, and reads as one rather than as a model that failed.
func TestConstraintEvaluationErrorIsNotAViolation(t *testing.T) {
	s := NewSession()
	s.Submit(`package Bad { constraint broken { nonexistent > 0 } }`)
	got := run(t, s, "%constraint Bad::broken")
	wants(t, got, "? Constraint Bad::broken could not be evaluated", "Error:")
	rejects(t, got, "Assertion evaluated to false", "Constraint Bad::broken failed", "✗")
}

// --- Debugger session lifetime across submissions ---

// An unrelated declaration must not silently end an in-progress debugging
// session: the next %step still works.
func TestUnrelatedSubmissionKeepsDebuggerAlive(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")

	res := s.Submit(`package Unrelated { part def Widget { attribute size = 1.0; } }`)
	if len(res.Notices) != 0 {
		t.Errorf("unrelated submission reported %v", res.Notices)
	}
	wants(t, run(t, s, "%step"), "✓ Step complete")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
}

// Redeclaring the declaration the debugged behavior lives under rewrites the
// graph being stepped, so the session ends — with a notice, rather than an
// unexplained failure on the next %step.
func TestRedeclarationEndsDebuggerWithNotice(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")

	res := s.Submit("package Debug {\n\taction tally {\n\t\tfirst start;\n\t\tthen done;\n\t}\n}")
	if !hasNotice(res, `action debugging session for "tally" ended`) {
		t.Fatalf("notices = %v, want an ended-session note", res.Notices)
	}
	if !strings.Contains(strings.Join(renderResult(res, VerbosityNormal), "\n"), "ended") {
		t.Error("notice was not rendered to the user")
	}
	wants(t, run(t, s, "%step"), "no active action session")
}

// A behavior typed straight at the prompt owns itself, so redeclaring it ends
// the session the same way redeclaring its package does.
func TestTopLevelRedeclarationEndsDebugger(t *testing.T) {
	const tally = "action tally {\n\tattribute total = 0;\n\taction accumulate {\n\t\tassign total := total + 5;\n\t}\n\tfirst start;\n\tthen accumulate;\n\tthen done;\n}"
	s := NewSession()
	if res := s.Submit(tally); len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	run(t, s, "%action tally")

	res := s.Submit(tally)
	if !hasNotice(res, `action debugging session for "tally" ended`) {
		t.Fatalf("notices = %v, want an ended-session note", res.Notices)
	}
	wants(t, run(t, s, "%step"), "no active action session")
}

// A behavior is invalidated by a change to what it depends on, not only by a
// change to itself: redeclaring a type its features are of rewrites what those
// features mean, so the session ends and names that declaration.
func TestDebuggerEndsWhenADeclarationItDependsOnChanges(t *testing.T) {
	s := NewSession()
	s.Submit("part def Kind { attribute size = 1.0; }")
	res := s.Submit("action tally {\n\tpart k : Kind;\n\tfirst start;\n\tthen done;\n}")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	run(t, s, "%action tally")

	res = s.Submit("part def Kind { attribute size = 2.0; }")
	// The declaration that moved is named, not the behavior the user left alone.
	if !hasNotice(res, `action debugging session for "tally" ended (Kind was redeclared)`) {
		t.Fatalf("notices = %v, want the session ended and Kind named as what changed", res.Notices)
	}
	wants(t, run(t, s, "%step"), "no active action session")
}

// A behavior performed by an object runs against that object, so a submission
// that drops it ends the session and says which object went.
func TestDebuggerEndsWhenItsPerformingObjectIsDropped(t *testing.T) {
	s := NewSession()
	s.Submit("part def Holder { attribute size = 1.0; }")
	res := s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	run(t, s, "%instantiate Holder")
	if started := run(t, s, "%action tally Holder"); !strings.Contains(started, "Started action executor") {
		t.Fatalf("%%action failed: %s", started)
	}

	res = s.Submit("part def Holder { attribute size = 2.0; }")
	if !hasNotice(res, `the object Holder performing it was dropped`) {
		t.Fatalf("notices = %v, want the performing object named", res.Notices)
	}
	wants(t, run(t, s, "%step"), "no active action session", "the object Holder performing it was dropped")
}

// The same contract for the state machine debugger.
func TestStateDebuggerSurvivesUnrelatedSubmission(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")
	started := run(t, s, "%state Cycle")
	if !strings.Contains(started, "Started state machine executor") {
		t.Fatalf("%%state failed: %s", started)
	}

	if res := s.Submit(`package Unrelated { part def Widget { attribute size = 1.0; } }`); len(res.Notices) != 0 {
		t.Errorf("unrelated submission reported %v", res.Notices)
	}
	rejects(t, run(t, s, "%current"), "no active")
}

// --- Instance lifetime across submissions ---

// A declaration that cannot affect an object leaves it alone. The object is
// carried into the resolution the submission produced, so it is still usable
// there rather than a stale pointer into the document it was built against.
func TestInstanceSurvivesUnrelatedDeclaration(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%instantiate Demo::Vehicle"), "ID: 1")

	if res := s.Submit("part def Widget;"); len(res.Notices) != 0 {
		t.Fatalf("unrelated declaration reported %v", res.Notices)
	}
	listing := run(t, s, "%instances")
	wants(t, listing, "Demo::Vehicle (ID: 1)")
	rejects(t, listing, "dropped")

	wants(t, run(t, s, "%features Demo::Vehicle"), "mass = 1500.0")
	wants(t, run(t, s, "%eval Demo::Vehicle::mass"), "1500.0")
	// The carried objects hold their identities in the new context — the vehicle
	// and the engine part inside it — so the next object built does not reuse one.
	wants(t, run(t, s, "%instantiate Demo::Engine"), "ID: 3")
}

// A connector its owner declares no name for costs the object nothing: it is
// materialized again in the resolution the submission produced, under the
// identity it had, so the object survives an unrelated declaration whole.
func TestInstanceOwningAnAnonymousConnectorSurvives(t *testing.T) {
	s := NewSession()
	s.Submit(`package Demo {
		port def P;
		part def A { port p : P; }
		part def B { port q : P; }
		part def Sys { part a : A; part b : B; connect a.p to b.q; }
	}`)
	wants(t, run(t, s, "%instantiate Demo::Sys"), "ID: 1")
	fvs := run(t, s, "%features Demo::Sys")
	wants(t, fvs, "(anonymous connector)")
	connector := connectorLine(t, fvs)

	for _, decl := range []string{"part def Widget;", "part def Gadget;"} {
		if res := s.Submit(decl); len(res.Notices) != 0 {
			t.Fatalf("%s reported %v", decl, res.Notices)
		}
		listing := run(t, s, "%instances")
		wants(t, listing, "Demo::Sys (ID: 1)")
		rejects(t, listing, "dropped")
		// The same connector of the same object is named the same, submission
		// after submission, rather than costing an identity each time.
		if got := connectorLine(t, run(t, s, "%features Demo::Sys")); got != connector {
			t.Errorf("after %s the connector reads %q, want %q", decl, got, connector)
		}
	}
	// Nothing else took the identity the connector kept.
	wants(t, run(t, s, "%instantiate Demo::A"), "ID: 7")

	// Submissions nothing reads the connectors between keep the identity too.
	s.Submit("part def Gizmo;")
	s.Submit("part def Doodad;")
	if got := connectorLine(t, run(t, s, "%features Demo::Sys")); got != connector {
		t.Errorf("after two unread submissions the connector reads %q, want %q", got, connector)
	}
}

// connectorLine returns the line of a %features listing that holds the object's
// anonymous connector.
func connectorLine(t *testing.T, listing string) string {
	t.Helper()
	for _, line := range strings.Split(listing, "\n") {
		if strings.Contains(line, "(anonymous connector)") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("no anonymous connector in:\n%s", listing)
	return ""
}

// An object is invalidated by a change to what its declaration depends on, not
// only by a change to that declaration: redeclaring a type one of its features
// is of rewrites what the object holds.
func TestInstanceDropsWhenADeclarationItDependsOnChanges(t *testing.T) {
	s := NewSession()
	s.Submit("part def Kind { attribute size = 1.0; }")
	s.Submit("part def Holder { part k : Kind; }")
	wants(t, run(t, s, "%instantiate Holder"), "ID: 1")

	res := s.Submit("part def Kind { attribute size = 2.0; }")
	if !hasNotice(res, "1 instance was dropped") {
		t.Fatalf("notices = %v, want the dropped instance counted", res.Notices)
	}
	wants(t, run(t, s, "%instances"), "no instances created", "1 instance was dropped")
}

// A value an object computed from an expression is computed again against the
// declarations that expression reads now, so a change to one of them updates the
// object rather than going unseen or costing it.
func TestInstanceRederivesAValueWhenAnExpressionItReadsChanges(t *testing.T) {
	s := NewSession()
	s.Submit("calc def double { in x; return : ScalarValues::Real = x * 2.0; }")
	s.Submit("part def A { attribute m = double(3.0); }")
	wants(t, run(t, s, "%instantiate A"), "ID: 1")
	wants(t, run(t, s, "%features A"), "m = 6.0")

	res := s.Submit("calc def double { in x; return : ScalarValues::Real = x * 3.0; }")
	if hasNotice(res, "instance was dropped") {
		t.Fatalf("notices = %v, want the object kept", res.Notices)
	}
	wants(t, run(t, s, "%features A"), "m = 9.0")
}

// A connector holds the features it connects rather than values of its own, so
// an end reads what the feature holds now: a change to what a connected value is
// computed from must not leave the end disagreeing with it.
func TestConnectorEndsAreReadAgainAfterADependencyChanges(t *testing.T) {
	s := NewSession()
	s.Submit("calc def double { in x; return : ScalarValues::Real = x * 2.0; }")
	s.Submit(`package Demo {
		part def A { attribute x = double(3.0); }
		part def B { attribute y = 1.0; }
		part def Sys { part a : A; part b : B; connection c1 connect a.x to b.y; }
	}`)
	wants(t, run(t, s, "%instantiate Demo::Sys"), "ID: 1")
	wants(t, run(t, s, "%features Demo::Sys"), "x = 6.0", "c1 = Instance(ID: 4)", "source = 6.0")

	s.Submit("calc def double { in x; return : ScalarValues::Real = x * 3.0; }")
	// The end reads the new value, under the identity the connector kept.
	wants(t, run(t, s, "%features Demo::Sys"), "x = 9.0", "c1 = Instance(ID: 4)", "source = 9.0")

	// Nothing else took that identity.
	wants(t, run(t, s, "%instantiate Demo::A"), "ID: 5")
}

// A collection holds copies of what the features subsetting it hold, so it is
// collected again when one of those is derived from a declaration that changed.
func TestCollectionIsCollectedAgainAfterADependencyChanges(t *testing.T) {
	s := NewSession()
	s.Submit("calc def double { in x; return : ScalarValues::Real = x * 2.0; }")
	s.Submit("part def A { attribute pool : ScalarValues::Real[*]; attribute one :> pool = double(3.0); }")
	wants(t, run(t, s, "%instantiate A"), "ID: 1")
	wants(t, run(t, s, "%features A"), "pool = [6.0]", "one = 6.0")

	s.Submit("calc def double { in x; return : ScalarValues::Real = x * 3.0; }")
	wants(t, run(t, s, "%features A"), "pool = [9.0]", "one = 9.0")

	// A collection of objects is kept, so its members keep their identities.
	s.Submit("package D { part def B; part def C { part xs : B[3]; } }")
	wants(t, run(t, s, "%instantiate D::C"), "ID:")
	held := run(t, s, "%features D::C")
	s.Submit("part def Widget;")
	if got := run(t, s, "%features D::C"); got != held {
		t.Errorf("the objects of the collection read\n%s\nwant\n%s", got, held)
	}
}

// A variation's default states which variant an object selects rather than a
// value to compute, so the object bound to it is the same object across a
// submission rather than one materialized again under a new identity.
func TestSelectedVariantKeepsItsIdentity(t *testing.T) {
	s := NewSession()
	s.Submit(`package Demo {
		part def Engine { attribute size = 1.0; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine;
				variant part petrol : Engine;
			}
		}
		part sedan :> family { part :>> engine = engine::electric; }
	}`)
	wants(t, run(t, s, "%instantiate Demo::sedan"), "ID: 1")
	wants(t, run(t, s, "%features Demo::sedan"), "engine = electric (Instance ID: 2)")

	if res := s.Submit("part def Widget;"); len(res.Notices) != 0 {
		t.Fatalf("an unrelated declaration reported %v", res.Notices)
	}
	wants(t, run(t, s, "%features Demo::sedan"), "engine = electric (Instance ID: 2)")
	wants(t, run(t, s, "%instantiate Demo::Engine"), "ID: 3")

	// A change to which variant is selected still invalidates the object.
	res := s.Submit(`package Demo {
		part def Engine { attribute size = 1.0; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine;
				variant part petrol : Engine;
			}
		}
		part sedan :> family { part :>> engine = engine::petrol; }
	}`)
	if !hasNotice(res, "dropped because the declarations changed") {
		t.Fatalf("notices = %v, want the object of the changed selection dropped", res.Notices)
	}
}

// The declarations an expression reads are reached through other expressions
// too, so a change two reads away is seen as well.
func TestInstanceRederivesAValueThroughAChainOfReads(t *testing.T) {
	s := NewSession()
	s.Submit("calc def inner { in x; return : ScalarValues::Real = x * 2.0; }")
	s.Submit("calc def outer { in y; return : ScalarValues::Real = inner(y) + 1.0; }")
	s.Submit("part def A { attribute m = outer(3.0); }")
	wants(t, run(t, s, "%instantiate A"), "ID: 1")
	wants(t, run(t, s, "%features A"), "m = 7.0")

	s.Submit("calc def inner { in x; return : ScalarValues::Real = x * 3.0; }")
	wants(t, run(t, s, "%features A"), "m = 10.0")

	s.Submit("attribute g = 5.0;")
	s.Submit("attribute h = g * 2.0;")
	s.Submit("part def B { attribute m = h + 1.0; }")
	wants(t, run(t, s, "%instantiate B"), "ID:")
	wants(t, run(t, s, "%features B"), "m = 11.0")

	s.Submit("attribute g = 7.0;")
	wants(t, run(t, s, "%features B"), "m = 15.0")
}

// A submission that invalidates some of what the session holds says so even
// though the rest survived: a listing of the survivors alone would read as
// though nothing went.
func TestInstancesReportsASurvivorAlongsideALoss(t *testing.T) {
	s := NewSession()
	s.Submit("part def A { attribute x = 1.0; }")
	s.Submit("part def B { attribute y = 2.0; }")
	wants(t, run(t, s, "%instantiate A"), "ID: 1")
	wants(t, run(t, s, "%instantiate B"), "ID: 2")

	res := s.Submit("part def B { attribute y = 3.0; }")
	if !hasNotice(res, "1 instance was dropped") {
		t.Fatalf("notices = %v, want one dropped instance counted", res.Notices)
	}
	listing := run(t, s, "%instances")
	wants(t, listing, "A (ID: 1)", "1 instance was also dropped")
	rejects(t, listing, "B (ID")
}

// A submission that carries nothing over still takes over the identities of a
// context a debugging session goes on materializing objects through.
func TestSubmissionKeepsTheIdentitiesADebuggedContextHandedOut(t *testing.T) {
	s := NewSession()
	s.Submit("part def Holder { attribute size = 1.0; }")
	res := s.Submit("action tally {\n\tattribute total = 0;\n\tfirst start;\n\tthen done;\n}")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	wants(t, run(t, s, "%instantiate Holder"), "ID: 1")
	if started := run(t, s, "%action tally"); !strings.Contains(started, "Started action executor") {
		t.Fatalf("%%action failed: %s", started)
	}

	// The object goes with its declaration, so nothing is carried over — but the
	// session still runs against the context that handed out identity 1.
	res = s.Submit("part def Holder { attribute size = 2.0; }")
	if !hasNotice(res, "1 instance was dropped") {
		t.Fatalf("notices = %v, want the redeclared object dropped", res.Notices)
	}
	rejects(t, run(t, s, "%tokens"), "no active")
	wants(t, run(t, s, "%instantiate Holder"), "ID: 2")
}

// A loss belongs to the submission that caused it: once a later one has taken
// nothing, the listing stops explaining a loss it no longer describes.
func TestInstancesStopsRepeatingAnOldLoss(t *testing.T) {
	s := NewSession()
	s.Submit("part def A { attribute x = 1.0; }")
	s.Submit("part def B { attribute y = 2.0; }")
	run(t, s, "%instantiate A")
	run(t, s, "%instantiate B")
	s.Submit("part def B { attribute y = 3.0; }")
	wants(t, run(t, s, "%instances"), "1 instance was also dropped")

	s.Submit("part def Widget;")
	listing := run(t, s, "%instances")
	wants(t, listing, "A (ID: 1)")
	rejects(t, listing, "dropped")
}

// The instances a submission invalidates are counted in a notice, so the
// objects created before it do not disappear without a word.
func TestSubmissionReportsTheInstancesItDropped(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Demo::Vehicle")
	run(t, s, "%instantiate Demo::Engine")

	// Redeclaring the package the two definitions live in rewrites both of them.
	res := s.Submit("package Demo { part def Engine; part def Vehicle; }")
	if !hasNotice(res, "2 instances were dropped") {
		t.Fatalf("notices = %v, want the dropped instances counted", res.Notices)
	}
	// A note reads as a consequence of the declaration it follows, so it comes
	// after the accepted line rather than before it.
	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	wants(t, out, "2 instances were dropped")
	if strings.Index(out, "✓ package Demo") > strings.Index(out, "note:") {
		t.Errorf("note printed before the declaration it followed from:\n%s", out)
	}
}

// A command that would drive an ended action session says which submission
// ended it, instead of reporting only that nothing is active.
func TestEndedActionSessionExplainsItselfToEveryCommand(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	s.Submit("package Debug {\n\taction tally {\n\t\tfirst start;\n\t\tthen done;\n\t}\n}")

	const why = `the action session for "tally" ended when Debug::tally was redeclared at submission 2`
	wants(t, run(t, s, "%step"), why)
	wants(t, run(t, s, "%tokens"), why)
	wants(t, run(t, s, "%continue"), why)
	wants(t, run(t, s, "%stop"), "ended when Debug::tally was redeclared at submission 2")

	// Starting a new session clears the explanation with it.
	run(t, s, "%action tally")
	rejects(t, run(t, s, "%step"), "no active action session")
}

// The same for the state machine debugger, whose commands are %current and
// %advance.
func TestEndedStateSessionExplainsItselfToEveryCommand(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")
	run(t, s, "%state Cycle")
	s.Submit("package Debug {\n\tstate Cycle {\n\t\tentry; then init;\n\t\tstate init;\n\tsuccession first tinit then done;\n\t}\n}")

	const why = `the state machine session for "Cycle" ended when Debug::Cycle was redeclared at submission 2`
	wants(t, run(t, s, "%current"), why)
	wants(t, run(t, s, "%advance 1"), why)
	wants(t, run(t, s, "%events"), why)
}

// A member of a nested part is answered against that part, not against the
// enclosing object that happens to be the instantiated one.
func TestNestedPartMemberBindsToTheNestedInstance(t *testing.T) {
	s := loadFixture(t, "testdata/nested_part.sysml")
	run(t, s, "%instantiate Nested::Car")

	got := run(t, s, "%eval Nested::Car::engine::mass")
	wants(t, got, "= 5.0")
	rejects(t, got, "on Nested::Car ID")
	wants(t, run(t, s, "%eval Nested::Car::mass"), "on Nested::Car ID", "= 1500.0")
	wants(t, run(t, s, "%constraint Nested::Car::engine::light"),
		"passed", "on Nested::Car.engine")
}

// A multi-valued feature shows what the object holds, not <unknown>.
func TestCollectionFeatureValuesShowTheirContents(t *testing.T) {
	s := loadFixture(t, "testdata/collection_slots.sysml")
	run(t, s, "%instantiate Coll::Rig")

	got := run(t, s, "%features Coll::Rig")
	wants(t, got, "doubles = [200.0]", "wheels = [Instance(ID: 2), Instance(ID: 3)]")
	rejects(t, got, "<unknown>")
	wants(t, run(t, s, "%eval Coll::Rig::doubles"), "= [200.0]")
}

// A part held in a feature value is worth nothing to the reader as an opaque ID: %features
// shows what the nested object holds, indented under the feature value that holds it.
func TestFeatureValuesExpandNestedInstances(t *testing.T) {
	s := loadFixture(t, "testdata/nested_part.sysml")
	run(t, s, "%instantiate Nested::Car")

	wants(t, run(t, s, "%features Nested::Car"),
		"  engine = Instance(ID: 2)",
		"    mass = 5.0",
		"    light: <constraint: satisfied>",
	)
}

// Each element of a multi-valued part feature value is expanded too.
func TestFeatureValuesExpandCollectionElements(t *testing.T) {
	s := loadFixture(t, "testdata/collection_slots.sysml")
	run(t, s, "%instantiate Coll::Rig")

	got := run(t, s, "%features Coll::Rig")
	wants(t, got, "wheels = [Instance(ID: 2), Instance(ID: 3)]")
	if strings.Count(got, "    radius") != 2 {
		t.Errorf("expected both wheels expanded, got:\n%s", got)
	}
}

// A part containing its own kind materializes a fresh object per expansion, so
// nesting is bounded by type rather than by instance identity.
func TestFeatureValuesStopAtRecursiveContainment(t *testing.T) {
	s := NewSession()
	s.Submit("part def Node { attribute v = 1.0; part child : Node; }")
	run(t, s, "%instantiate Node")

	got := run(t, s, "%features Node")
	wants(t, got, "v = 1.0", "child : Node (not expanded: contains its own kind)")
	// The listing stops at the node itself: nothing is expanded under it.
	if n := strings.Count(got, "child"); n != 1 {
		t.Errorf("expected a bounded listing, got %d children:\n%s", n, got)
	}
	for _, line := range strings.Split(got, "\n")[2:] {
		if strings.HasPrefix(line, "    ") {
			t.Errorf("expected no nested expansion, got %q in:\n%s", line, got)
		}
	}
}

// Nesting multiplies, and every expansion materializes an object, so a wide
// model is truncated rather than listed in full.
func TestFeatureValuesTruncateWideNesting(t *testing.T) {
	s := NewSession()
	s.Submit("part def Leaf { attribute v = 1.0; } part def Mid { part leaves : Leaf[20]; } part def Top { part mids : Mid[20]; }")
	run(t, s, "%instantiate Top")

	got := run(t, s, "%features Top")
	wants(t, got, "… (listing truncated; %features Top all shows it whole, %features Top depth <n> to a depth)")
	if n := strings.Count(got, "\n"); n > maxFeatureValueLines+10 {
		t.Errorf("listing ran to %d lines, want it bounded near %d:\n%.400s", n, maxFeatureValueLines, got)
	}
}

// Adding a member to a package leaves the rest of its body as it was, so a
// debugging session over another member of it keeps running.
func TestDebugSessionSurvivesAnAdditionToItsPackage(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	res := s.Submit("package Debug { part def Widget; }")

	if hasNotice(res, "debugging session") {
		t.Errorf("notices = %v, want the untouched session kept", res.Notices)
	}
	wants(t, run(t, s, "%tokens"), "Active tokens")
}
