package repl

import "testing"

// ownedConstraintFixture declares named require/assume constraints with a body,
// typed by a constraint definition, and redefined by a specialization under the
// same name (a requirement constraint takes no name from what it redefines). A
// redefinition keeps the inherited condition (KerML 8.3.4.8): one tightening it
// nests an assertion beside it rather than stating a body of its own.
const ownedConstraintFixture = `package Power {
    constraint def Positive { 600.0 > 0.0 }
    constraint def Shortfall { 300.0 >= 450.0 }
    requirement def Margin {
        require constraint enough { 600.0 >= 450.0 }
        require constraint tooLittle { 300.0 >= 450.0 }
        assume constraint supplied : Positive;
        require constraint typed : Shortfall;
    }
    requirement def Tight :> Margin {
        require constraint enough :>> enough { assert constraint { 600.0 >= 650.0 } }
    }
    requirement def Ample {
        require constraint enough { 600.0 >= 450.0 }
        assume constraint supplied : Positive;
    }
    requirement def Kept :> Margin {
        require constraint tooLittle :>> tooLittle;
        require constraint typed :>> typed;
    }
    requirement def Braced :> Margin {
        require constraint tooLittle :>> tooLittle { }
        require constraint typed :>> typed { doc /* still the shortfall rule */ }
    }
    requirement def Nested :> Margin {
        require constraint tooLittle :>> tooLittle { assert constraint { } }
        require constraint typed :>> typed { assert constraint { doc /* no condition */ } }
        require constraint enough :>> enough { assert constraint { 600.0 >= 650.0 } }
    }
    requirement def Pair {
        require constraint low { 300.0 >= 450.0 }
        require constraint bare;
    }
    requirement def Joint :> Pair {
        require constraint :>> low, bare;
    }
}
`

// TestCheckNamedOwnedConstraint checks a named require/assume constraint by
// qualified name like any other constraint usage.
func TestCheckNamedOwnedConstraint(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckConstraint("Power::Margin::enough"), VerdictHolds,
		"✓ Constraint Power::Margin::enough passed")
	wantVerdict(t, s.CheckConstraint("Power::Margin::tooLittle"), VerdictFails,
		"✗ Constraint Power::Margin::tooLittle failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Margin::supplied"), VerdictHolds,
		"✓ Constraint Power::Margin::supplied passed")
	wantVerdict(t, s.CheckConstraint("Power::Margin::typed"), VerdictFails,
		"✗ Constraint Power::Margin::typed failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Tight::enough"), VerdictFails,
		"✗ Constraint Power::Tight::enough failed",
		"Assertion evaluated to false: 600.0 >= 650.0")
	wantVerdict(t, s.CheckConstraint("Power::Margin"), VerdictUnresolved, "not a constraint")
}

// TestCheckRedefinedNamedOwnedConstraint checks that a redefinition stating no
// condition (`;`, `{ }`, a doc-only body) inherits the redefined one's.
func TestCheckRedefinedNamedOwnedConstraint(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckConstraint("Power::Kept::tooLittle"), VerdictFails,
		"✗ Constraint Power::Kept::tooLittle failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Kept::typed"), VerdictFails,
		"✗ Constraint Power::Kept::typed failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Braced::tooLittle"), VerdictFails,
		"✗ Constraint Power::Braced::tooLittle failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Braced::typed"), VerdictFails,
		"✗ Constraint Power::Braced::typed failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
}

// TestCheckNestedConstraintKeepsRedefinedResult checks that a redefinition whose
// body only nests a constraint — empty, doc-only or stating a condition — owns no
// result expression, so the redefined one is still checked alongside it.
func TestCheckNestedConstraintKeepsRedefinedResult(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckConstraint("Power::Nested::tooLittle"), VerdictFails,
		"✗ Constraint Power::Nested::tooLittle failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Nested::typed"), VerdictFails,
		"✗ Constraint Power::Nested::typed failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Nested::enough"), VerdictFails,
		"✗ Constraint Power::Nested::enough failed",
		"Assertion evaluated to false: 600.0 >= 650.0")
	wantVerdict(t, s.CheckRequirement("Power::Nested"), VerdictFails,
		"✗ Requirement Power::Nested failed",
		"Required condition evaluated to false: 300.0 >= 450.0")
}

// TestCheckAnonymousRedefinitionOfTwoConstraints checks that a constraint no
// name names, redefining two, is checked as what it redefines.
func TestCheckAnonymousRedefinitionOfTwoConstraints(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckRequirement("Power::Joint"), VerdictFails,
		"✗ Requirement Power::Joint failed",
		"Required condition evaluated to false: 300.0 >= 450.0")
}

// TestCheckRequirementWithNamedOwnedConstraints checks that a requirement
// requires what its named constraints state, typing definitions included.
func TestCheckRequirementWithNamedOwnedConstraints(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckRequirement("Power::Ample"), VerdictHolds,
		"✓ Requirement Power::Ample satisfied")
	wantVerdict(t, s.CheckRequirement("Power::Margin"), VerdictFails,
		"✗ Requirement Power::Margin failed",
		"Required condition evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckRequirement("Power::Tight"), VerdictFails,
		"✗ Requirement Power::Tight failed")
}

// TestCheckRequirementMasksRedefinedNamedConstraints checks that a requirement
// requires each redefined constraint once, through the redefinition: the
// inherited body a redefinition stating no condition (`;`, `{ }`, a doc-only
// body) keeps, and not the redefined usage a second time.
func TestCheckRequirementMasksRedefinedNamedConstraints(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckRequirement("Power::Kept"), VerdictFails,
		"✗ Requirement Power::Kept failed",
		"Required condition evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckRequirement("Power::Braced"), VerdictFails,
		"✗ Requirement Power::Braced failed",
		"Required condition evaluated to false: 300.0 >= 450.0")
}
