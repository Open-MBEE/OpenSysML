package repl

import (
	"strings"
	"testing"
)

// TestAnalysisRunsADefinitionOwnedCaseOnTheObjectGiven checks that the object
// named after a case a part definition owns is the object the case is a feature
// of: its body reads that object's cost, and the verdict names the object.
func TestAnalysisRunsADefinitionOwnedCaseOnTheObjectGiven(t *testing.T) {
	s := loadFixture(t, "testdata/verification_definition_owned.sysml")
	run(t, s, "%instantiate Cost::cheap")
	run(t, s, "%instantiate Cost::dear")
	wants(t, run(t, s, "%analysis Cost::Definition::costTest Cost::cheap"),
		"verdict = [VerdictKind::pass]",
		"✓ Verification Cost::Definition::costTest on Cost::cheap (object #1) verdict: pass")
	wants(t, run(t, s, "%analysis Cost::Definition::costTest Cost::dear"),
		"verdict = [VerdictKind::fail]",
		"✗ Verification Cost::Definition::costTest on Cost::dear (object #2) verdict: fail")
	// Two objects of the definition and none named: the case is not run on one
	// picked at random.
	wants(t, run(t, s, "%analysis Cost::Definition::costTest"), "ambiguous subject", "cheap", "dear")
}

// TestAnalysisRunsADefinitionOwnedCaseOnTheObjectGivenOverOneHeld checks that
// an object held under the definition's own name does not displace the object
// named: the body reads the named object's cost, and a body that fails on an
// object still names it.
func TestAnalysisRunsADefinitionOwnedCaseOnTheObjectGivenOverOneHeld(t *testing.T) {
	s := loadFixture(t, "testdata/verification_definition_owned.sysml")
	run(t, s, "%instantiate Cost::Definition")
	run(t, s, "%instantiate Cost::dear")
	wants(t, run(t, s, "%analysis Cost::Definition::costTest Cost::dear"),
		"verdict = [VerdictKind::fail]",
		"✗ Verification Cost::Definition::costTest on Cost::dear (object #2) verdict: fail")
	wants(t, run(t, s, "%analysis Cost::Definition::costTest"),
		"? Verification Cost::Definition::costTest on Instance(ID: 1) verdict: error", "no value for feature cost")
}

// TestAnalysisOfADefinitionOwnedCaseWithoutAnObject checks that the case runs
// on the one object of the definition the session holds, and asks for one when
// it holds none, as a definition's satisfy does, rather than failing on cost.
func TestAnalysisOfADefinitionOwnedCaseWithoutAnObject(t *testing.T) {
	s := loadFixture(t, "testdata/verification_definition_owned.sysml")
	none := run(t, s, "%analysis Cost::Definition::costTest")
	wants(t, none, "? Verification Cost::Definition::costTest verdict: inconclusive — needs an object of Cost::Definition: none is instantiated")
	if strings.Contains(none, "error") {
		t.Errorf("a case with no object to run on is an error:\n%s", none)
	}
	run(t, s, "%instantiate Cost::dear")
	wants(t, run(t, s, "%analysis Cost::Definition::costTest"),
		"✗ Verification Cost::Definition::costTest on Cost::dear (object #1) verdict: fail")
}

// TestChecksReportADefinitionOwnedCasePerObject checks that %validate,
// %requirement and %satisfy run the case on the object the check is about.
func TestChecksReportADefinitionOwnedCasePerObject(t *testing.T) {
	s := loadFixture(t, "testdata/verification_definition_owned.sysml")
	run(t, s, "%instantiate Cost::cheap")
	run(t, s, "%instantiate Cost::dear")
	wants(t, run(t, s, "%validate Cost::cheap"),
		"✓ satisfy costReq holds (on Cost::cheap ID: 1)",
		"✓ Verification Cost::Definition::costTest on Cost::cheap (object #1) verdict: pass",
		"✓ Cost::cheap is valid")
	wants(t, run(t, s, "%validate Cost::dear"),
		"✗ satisfy costReq fails (on Cost::dear ID: 2)",
		"✗ Verification Cost::Definition::costTest on Cost::dear (object #2) verdict: fail")
	got := run(t, s, "%requirement Cost::costReq Cost::dear")
	wants(t, got, "✗ Verification Cost::Definition::costTest on Cost::dear (object #2) verdict: fail")
	if strings.Contains(got, "Cost::cheap") {
		t.Errorf("%%requirement on dear reports cheap's verdict too:\n%s", got)
	}

	one := loadFixture(t, "testdata/verification_definition_owned.sysml")
	run(t, one, "%instantiate Cost::cheap")
	wants(t, run(t, one, "%satisfy"),
		"✓ satisfy costReq holds (on Cost::cheap ID: 1)",
		"✓ Verification Cost::Definition::costTest on Cost::cheap (object #1) verdict: pass")
}
