// A requirement's `assume c;` / `require c;` naming a constraint declared elsewhere — in
// the enclosing package or as a feature of the requirement definition — is a member of
// `assumptions` / `constraints` too (SysML v2 §7.21.2, §8.3.20.4), its `result` the named
// constraint checked on the requirement, so a false external assumption reads false in
// the collection as it does in the requirement's own result (#1854).
package test {
	private import ScalarValues::Boolean;
	private import ControlFunctions::allTrue;

	constraint never { false }
	requirement def Guarded {
		constraint inner { true }
		assume inner;
		require constraint always { true }
	}
	requirement def External { assume never; require constraint yes { true } }
	part sys {
		requirement external : External;
		requirement guarded : Guarded;
	}

	calc probe { out one : Boolean = true; }

	attribute externalAssumptions : Boolean[0..*] = sys.external.assumptions.result;
	attribute externalAllTrue : Boolean = allTrue(sys.external.assumptions());
	attribute externalResult : Boolean = sys.external.result;
	attribute externalConstraints : Boolean[0..*] = sys.external.constraints.result;
	attribute guardedAssumptions : Boolean[0..*] = sys.guarded.assumptions.result;
	attribute guardedAllTrue : Boolean = allTrue(sys.guarded.assumptions());
	attribute guardedConstraints : Boolean[0..*] = sys.guarded.constraints.result;
	attribute guardedResult : Boolean = sys.guarded.result;
}
