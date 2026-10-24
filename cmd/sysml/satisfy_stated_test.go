package main

import (
	"strings"
	"testing"
)

// statedSatisfyFiles state a satisfaction in a definition, with no `by`, across
// three files, and verify the same requirement from an objective stating
// nothing else.
var statedSatisfyFiles = map[string]string{
	"reqrmnt.sysml": `package RequirementPkg {
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
}`,
	"definition.sysml": `package DefinitionPkg {
    private import ScalarValues::Real;
    private import RequirementPkg::*;

    part def Definition {
        attribute cost : Real;
        satisfy costReq;
    }
}`,
	"product.sysml": `package ProductPkg {
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
}`,
}

// TestSatisfyStatedByADefinition checks -satisfy over a definition's `satisfy r;`:
// it is decided per object the run created, asks for one when there is none, and
// the objective's `verify` is never read as an assertion.
func TestSatisfyStatedByADefinition(t *testing.T) {
	binary := buildCLI(t)

	none := checkFiles(t, binary, statedSatisfyFiles, "-satisfy")
	wantReport(t, none, 2,
		"? satisfy costReq needs an object of DefinitionPkg::Definition: none is instantiated",
		"✓ Verification ProductPkg::costTest verdict: pass")
	if strings.Contains(none.output(), "could not be evaluated") {
		t.Errorf("an assertion was evaluated against no object, or a verify as one:\n%s", none.output())
	}

	wantReport(t, checkFiles(t, binary, statedSatisfyFiles, "-instantiate", "ProductPkg::product", "-satisfy"), 0,
		"✓ satisfy costReq holds (on ProductPkg::product ID: 1)")
	both := checkFiles(t, binary, statedSatisfyFiles,
		"-instantiate", "ProductPkg::product", "-instantiate", "ProductPkg::costly", "-satisfy")
	wantReport(t, both, 1,
		"✓ satisfy costReq holds (on ProductPkg::product ID: 1)",
		"✗ satisfy costReq fails (on ProductPkg::costly ID: 2)")
	if strings.Contains(both.output(), "could not be evaluated") {
		t.Errorf("the sweep did not decide every object:\n%s", both.output())
	}
}
