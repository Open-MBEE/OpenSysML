package repl

import (
	"strings"
	"testing"
)

// TestSatisfyVerdicts checks the entry point an anonymous satisfaction assertion
// is reached through: the element that states it, or the whole session.
func TestSatisfyVerdicts(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_landing.sysml")

	all := run(t, s, "%satisfy")
	wants(t, all,
		"✓ satisfy touchdown by slowLander holds",
		"✗ satisfy touchdown by fastLander fails",
		"Required condition evaluated to false: lander.verticalSpeed <= maxVerticalSpeed",
		"✓ not satisfy touchdown by fastLander holds",
	)

	wants(t, run(t, s, "%satisfy Landing::analysisContext"), "✓ satisfy touchdown by slowLander holds")
	wants(t, run(t, s, "%satisfy nosuch"), `unresolved reference: nosuch`)
	// A requirement is not itself an assertion, and states none.
	wants(t, run(t, s, "%satisfy touchdown"), "no satisfaction assertion in Landing::touchdown")

	empty := NewSession()
	wants(t, run(t, empty, "%satisfy"), "no declarations loaded")
}

// TestSatisfyReportsNothingCheckedAsUndecided checks that a session stating no
// assertion answers as the prompt answers any check it could not make, so a
// caller outside the prompt reports it the way it reports the others.
func TestSatisfyReportsNothingCheckedAsUndecided(t *testing.T) {
	s := NewSession()
	s.Submit("part def A;")

	wants(t, run(t, s, "%satisfy"), "error: no satisfaction assertion in the session")
	for _, v := range s.CheckSatisfy("") {
		if v.Status != VerdictUnresolved {
			t.Errorf("status = %v, want unresolved", v.Status)
		}
	}
}

// TestSatisfyUsesInstantiatedSubject checks that a verdict is about the object
// the session created for the subject, not a fresh one.
func TestSatisfyUsesInstantiatedSubject(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_landing.sysml")
	wants(t, run(t, s, "%instantiate Landing::slowLander"), "✓ Created instance of Landing::slowLander")
	wants(t, run(t, s, "%satisfy Landing::analysisContext"),
		"✓ satisfy touchdown by slowLander holds (on Landing::slowLander ID: 1)")
}

// TestRepeatedSatisfyKeepsItsSubject checks that a subject created by %satisfy
// itself is kept, so repeating the command is about the same object.
func TestRepeatedSatisfyKeepsItsSubject(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_landing.sysml")
	first := "✓ satisfy touchdown by slowLander holds (on Landing::slowLander ID: 1)"
	wants(t, run(t, s, "%satisfy Landing::analysisContext"), first)
	wants(t, run(t, s, "%satisfy Landing::analysisContext"), first)
	wants(t, run(t, s, "%features Landing::slowLander"), "verticalSpeed")
}

// TestSatisfyQuotesInnerNames checks that the prose naming an assertion quotes
// each inner name the notation needs quotes for, the way every other name the
// prompt prints is quoted.
func TestSatisfyQuotesInnerNames(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_quoted.sysml")
	out := run(t, s, "%satisfy")
	wants(t, out,
		"✓ satisfy 'the touchdown' by 'slow craft' holds (on 'Landing Site'::'slow craft' ID: 1)",
		"✗ satisfy 'the touchdown' by 'fast craft' fails",
		"✓ not satisfy 'the touchdown' by 'fast craft' holds",
	)
	rejects(t, out, "satisfy the touchdown by slow craft")
	// The verdict a caller outside the prompt reads is named the same way.
	for _, v := range s.CheckSatisfy("") {
		if strings.Contains(v.Subject, "the touchdown by") {
			t.Errorf("verdict subject %q is unquoted", v.Subject)
		}
	}
}

// TestSatisfyChainedSubject checks that a `by` operand written as a feature
// chain is evaluated on the nested object reached through its owner, is
// reported under the chain as written, and keeps that object so a repeated
// command is about the same one.
func TestSatisfyChainedSubject(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_chain.sysml")
	out := run(t, s, "%satisfy")
	wants(t, out,
		"✓ satisfy r1 by direct holds (on S::direct ID: 1)",
		"✓ satisfy r2 by config.child holds (on S::config.child ID: 3)",
	)
	rejects(t, out, "by child")
	wants(t, run(t, s, "%satisfy"), "✓ satisfy r2 by config.child holds (on S::config.child ID: 3)")
	wants(t, run(t, s, "%features S::config::child"), "mass")

	// A quoted member keeps its quotes, so a dot inside it is not a chain step.
	s.Submit("package Q { private import S::*; part def Box { part 'inner.part' : Part; } part box : Box; requirement r4 : MassReq { attribute :>> m = box.'inner.part'.mass; } satisfy r4 by box.'inner.part'; }")
	wants(t, run(t, s, "%satisfy Q"), "✓ satisfy r4 by box.'inner.part' holds (on Q::box.'inner.part' ID: ")

	// A chain whose segment resolves to nothing is reported as written.
	s.Submit("package N { private import S::*; requirement r3 : MassReq; satisfy r3 by config.nope; }")
	wants(t, run(t, s, "%satisfy N"),
		"? satisfy r3 by config.nope could not be evaluated",
		"satisfy r3 by config.nope: no subject to satisfy the requirement: config.nope",
	)
}
