package repl

import (
	"strings"
	"testing"
)

// statedSatisfyModel states a satisfaction in a definition, with no `by`, and
// verifies the same requirement from an objective that states nothing else.
const statedSatisfyModel = `
package RequirementPkg {
	private import ScalarValues::Real;
	private import DefinitionPkg::Definition;

	requirement def CostRequirement {
		subject product : Definition;
		attribute maximumCost : Real;
		require constraint costOk { product.cost <= maximumCost }
	}
	requirement costReq : CostRequirement {
		:>> maximumCost = 100.0;
	}
}
package DefinitionPkg {
	private import ScalarValues::Real;
	private import RequirementPkg::*;

	part def Definition {
		attribute cost : Real;
		satisfy costReq;
	}
}
package ProductPkg {
	private import DefinitionPkg::*;
	private import RequirementPkg::*;

	part product : Definition {
		attribute :>> cost = 80.0;
	}
	part costly : Definition {
		attribute :>> cost = 120.0;
	}

	verification def CostTest {
		subject p : Definition;
		objective costObjective { verify costReq; }
		return verdict = VerificationCases::PassIf(p.cost <= 100.0);
	}
	verification costTest : CostTest { subject p = product; }
	verification costlyTest : CostTest { subject p = costly; }
}
`

// TestSatisfyStatedByADefinitionSweepsItsObjects checks that %satisfy decides a
// definition's `satisfy r;` once per object of the definition the session holds,
// asks for one when there is none, and never reads a `verify` as an assertion.
func TestSatisfyStatedByADefinitionSweepsItsObjects(t *testing.T) {
	s := loadSource(t, statedSatisfyModel)

	none := run(t, s, "%satisfy")
	wants(t, none,
		"? satisfy costReq needs an object of DefinitionPkg::Definition: none is instantiated",
		"✓ Verification ProductPkg::costTest verdict: pass",
		"✗ Verification ProductPkg::costlyTest verdict: fail")
	if strings.Contains(none, "could not be evaluated") {
		t.Errorf("an assertion was evaluated against no object, or a verify as one:\n%s", none)
	}

	// The verification runs above made objects of their own, so the ids are read back.
	product := objectIDIn(t, run(t, s, "%instantiate ProductPkg::product"))
	costly := objectIDIn(t, run(t, s, "%instantiate ProductPkg::costly"))
	swept := run(t, s, "%satisfy")
	wants(t, swept,
		"✓ satisfy costReq holds (on ProductPkg::product ID: "+product+")",
		"✗ satisfy costReq fails (on ProductPkg::costly ID: "+costly+")",
		"Required condition evaluated to false: product.cost <= maximumCost")
	if strings.Contains(swept, "could not be evaluated") || strings.Contains(swept, "needs an object") {
		t.Errorf("the sweep did not decide every object:\n%s", swept)
	}
	if n := strings.Count(swept, "satisfy costReq holds") + strings.Count(swept, "satisfy costReq fails"); n != 2 {
		t.Errorf("%d satisfaction verdicts, want one per object:\n%s", n, swept)
	}
	wants(t, run(t, s, "%satisfy DefinitionPkg::Definition"), "✓ satisfy costReq holds (on ProductPkg::product ID: "+product+")")

	// A second object of product displaces the first from its name, and both
	// are objects of the definition, so both are decided.
	again := objectIDIn(t, run(t, s, "%instantiate ProductPkg::product"))
	twice := run(t, s, "%satisfy DefinitionPkg::Definition")
	wants(t, twice,
		"✓ satisfy costReq holds (on #"+product+" ID: "+product+")",
		"✓ satisfy costReq holds (on ProductPkg::product ID: "+again+")",
		"✗ satisfy costReq fails (on ProductPkg::costly ID: "+costly+")")
}

// TestSatisfyLabelsObjectsAsValidateDoes checks that %satisfy names each object a
// definition's `satisfy r;` is decided on by the indexed path %validate prints for it.
func TestSatisfyLabelsObjectsAsValidateDoes(t *testing.T) {
	s := loadSource(t, `package test {
	private import ScalarValues::Real;
	requirement def CostRequirement {
		subject product : Definition;
		attribute maximumCost : Real;
		require constraint costOk { product.cost <= maximumCost }
	}
	requirement costReq : CostRequirement { :>> maximumCost = 100.0; }
	part def Definition {
		attribute cost : Real;
		satisfy costReq;
	}
	part def Fleet { part cheap : Definition [2] { attribute :>> cost = 80.0; } }
	part fleet : Fleet;
}`)
	run(t, s, "%instantiate fleet")
	labels := []string{
		"✓ satisfy costReq holds (on test::fleet.cheap[1] ID: 2)",
		"✓ satisfy costReq holds (on test::fleet.cheap[2] ID: 3)",
	}
	satisfied := run(t, s, "%satisfy")
	wants(t, satisfied, labels...)
	wants(t, run(t, s, "%validate fleet"), labels...)
	if strings.Contains(satisfied, "fleet.cheap ID") {
		t.Errorf("%%satisfy labels an object by its feature alone:\n%s", satisfied)
	}
}
