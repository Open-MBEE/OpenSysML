package repl

import "testing"

// TestSatisfyBindsReferencedRequirementArguments checks that a satisfaction of
// a requirement whose `require r { in w = unit; }` members pass the subject on
// is decided from the attributes those requirements compute from it.
func TestSatisfyBindsReferencedRequirementArguments(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_freight.sysml")

	wants(t, run(t, s, "%satisfy"),
		"✗ satisfy wagonSpec by wagon1 fails",
		"Required condition evaluated to false: actual <= cap",
		"✓ satisfy wagonSpec by wagon2 holds",
	)

	// The referenced requirement on its own binds no subject, so it is undecided.
	wants(t, run(t, s, "%requirement Freight::ladenLimit"),
		"? Requirement Freight::ladenLimit could not be evaluated",
		"w subject is unbound")

	wants(t, run(t, s, "%instantiate Freight::wagon2"), "✓ Created instance of Freight::wagon2")
	wants(t, run(t, s, "%validate Freight::wagon2"),
		"✓ satisfy wagonSpec by wagon2 holds",
		"✓ Freight::wagon2 is valid")
}
