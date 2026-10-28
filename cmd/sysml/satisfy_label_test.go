package main

import (
	"strings"
	"testing"
)

// repeatedPartModel holds two objects of a definition stating a satisfaction
// under one usage with a multiplicity.
const repeatedPartModel = `package test {
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
}`

// TestSatisfyLabelsObjectsAsValidateDoes checks that -satisfy names each object a
// definition's `satisfy r;` is decided on by the indexed path -validate prints for it.
func TestSatisfyLabelsObjectsAsValidateDoes(t *testing.T) {
	binary := buildCLI(t)
	files := map[string]string{"fleet.sysml": repeatedPartModel}
	labels := []string{
		"✓ satisfy costReq holds (on test::fleet.cheap[1] ID: 2)",
		"✓ satisfy costReq holds (on test::fleet.cheap[2] ID: 3)",
	}
	satisfied := checkFiles(t, binary, files, "-instantiate", "test::fleet", "-satisfy")
	wantReport(t, satisfied, 0, labels...)
	validated := checkFiles(t, binary, files, "-instantiate", "test::fleet", "-validate=test::fleet")
	wantReport(t, validated, 0, labels...)
	if strings.Contains(satisfied.output(), "fleet.cheap ID") {
		t.Errorf("-satisfy labels an object by its feature alone, not its indexed path:\n%s", satisfied.output())
	}
}
