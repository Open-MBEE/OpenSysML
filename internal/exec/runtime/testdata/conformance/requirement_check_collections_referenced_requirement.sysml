// A requirement's `assume r;` / `require r;` naming a requirement is a member of the
// collection whose `result` is that requirement's own check, its implication included
// (SysML v2 §7.21.2; Requirements.sysml `RequirementConstraintCheck::result`): a
// referenced requirement whose required constraint is false reads false, one whose
// assumption is false reads true (vacuous), as `r.result` does (#1854).
package test {
	private import ScalarValues::Boolean;
	private import ControlFunctions::allTrue;

	requirement def Bad { require constraint bad { false } }
	requirement def Vacuous { assume constraint no { false } require constraint bad { false } }
	requirement def Outer {
		requirement r : Bad;
		requirement v : Vacuous;
		assume r;
		assume v;
		require constraint ok { true }
	}
	part sys { requirement outer : Outer; }

	calc probe { out one : Boolean = true; }

	attribute assumptions : Boolean[0..*] = sys.outer.assumptions.result;
	attribute assumptionsAllTrue : Boolean = allTrue(sys.outer.assumptions());
	attribute outerResult : Boolean = sys.outer.result;
	attribute badResult : Boolean = sys.outer.r.result;
	attribute vacuousResult : Boolean = sys.outer.v.result;
}
