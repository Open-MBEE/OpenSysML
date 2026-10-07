// SPDX-License-Identifier: Apache-2.0

package runtime

import "testing"

func TestImportedUnitNamesConflictWithFunctions(t *testing.T) {
	_, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package UnitNames {
		private import SI::*;
		private import QuantityCalculations::*;
		private import TrigFunctions::*;
	}`))
	pkg := lookupOne(t, ctx.Resolver().Index(), "UnitNames")
	for _, name := range []string{"min", "rad"} {
		if sym, ok := ctx.Resolver().LookupName(pkg.Scope, name); ok {
			t.Fatalf("%s must hide the unit/function clash, got %v", name, sym)
		}
	}
	for _, expr := range []string{"1.0 [SI::min]", "sin(0.5 [SI::rad])", "QuantityCalculations::min(1 [SI::m], 2 [SI::m])"} {
		if _, err := evalIn(t, ctx, pkg.Scope, expr); err != nil {
			t.Errorf("qualified unit or function %s: %v", expr, err)
		}
	}
}
