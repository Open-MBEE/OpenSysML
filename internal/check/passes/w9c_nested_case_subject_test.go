package passes

import "testing"

// Inside a case usage a part definition owns, `self` is the case's own
// `Performance::self` — the case, not the part — so binding the subject to it
// is a non-conforming binding, while `this` is the owning object
// (`Parts::Part::ownedActions::this`) and conforms. The pinned pilot agrees
// line for line.
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
