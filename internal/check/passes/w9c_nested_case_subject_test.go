package passes

import "testing"

// In a case usage a part def owns, `self` is the case's own `Performance::self`
// (not the part) and does not conform; `this` is the owning object and does.
func TestW9CDefinitionOwnedCaseSubjectSelfAndThis(t *testing.T) {
	src := `package Test {
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
		verification def CostTest {
			subject product : Definition;
			objective { verify costReq; }
			return verdict = VerificationCases::PassIf(product.cost <= costReq.maximumCost);
		}
		verification bySelf : CostTest { subject product = self; }
		verification byThis : CostTest { subject product = this; }
	}
}`
	w9cWantLines(t, src, "bound-feature-types", 17)
}
