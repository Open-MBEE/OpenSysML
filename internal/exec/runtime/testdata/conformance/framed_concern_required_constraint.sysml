// A framed concern is a subrequirement, a required constraint of the requirement framing it, reading its subject where it declares none,
// whether declared in place, typed or framed by reference (SysML v2 §7.21.3; `concerns :> concernChecks, subrequirements`).
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::size;
	part def Veh { attribute mass : Real; }
	concern def Weight { subject v : Veh; require constraint { v.mass < 10.0 } }
	concern weight : Weight;
	requirement def Typed { subject v : Veh; frame concern w : Weight; }
	requirement def Referenced { subject v : Veh; frame weight; }
	requirement def Inline { subject v : Veh; frame concern inl { require constraint { v.mass < 10.0 } } }
	part veh : Veh { attribute :>> mass = 20.0; }
	part feather : Veh { attribute :>> mass = 5.0; }
	part box {
		requirement typed : Typed { subject v = veh; }
		requirement typedOk : Typed { subject v = feather; }
		requirement referenced : Referenced { subject v = veh; }
		requirement referencedOk : Referenced { subject v = feather; }
		requirement inline : Inline { subject v = veh; }
		requirement inlineOk : Inline { subject v = feather; }
	}
	attribute typedResult = box.typed.result;
	attribute typedOkResult = box.typedOk.result;
	attribute typedConcern = box.typed.w.result;
	attribute typedOkConcern = box.typedOk.w.result;
	attribute typedConcernMass = box.typed.w.v.mass;
	attribute typedConcerns = size(box.typed.concerns);
	attribute typedSubrequirements = size(box.typed.subrequirements);
	attribute referencedResult = box.referenced.result;
	attribute referencedOkResult = box.referencedOk.result;
	attribute inlineResult = box.inline.result;
	attribute inlineOkResult = box.inlineOk.result;
	attribute inlineConcern = box.inline.inl.result;
	attribute inlineOkConcern = box.inlineOk.inl.result;
	calc probe { return one = 1; }
}
