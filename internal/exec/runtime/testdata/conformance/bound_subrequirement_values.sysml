// The check of a requirement ranges over the values bound to a subrequirement (SysML v2 §8.4.17.1; Requirements.sysml
// `subrequirements :> requirementChecks, constraints`, `result = allTrue(assumptions()) implies allTrue(constraints())`):
// no value states nothing, one value one check, several one each, each judged on the value with the container's subject.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::size;
	part def Veh { attribute mass : Real; }
	part veh : Veh { attribute :>> mass = 20.0; }
	requirement def Light { subject v : Veh; require constraint { v.mass < 10.0 } }
	requirement def Heavy { subject v : Veh; require constraint { v.mass > 10.0 } }
	requirement def Limit { subject v : Veh; attribute max : Real; require constraint { v.mass < max } }
	requirement def Tight :> Heavy { require constraint { v.mass > 50.0 } }
	requirement def Empty { subject v : Veh; requirement s[0..1] : Light = (); }
	requirement def Null { subject v : Veh; requirement s[0..1] : Light = null; }
	requirement def Unnamed { subject v : Veh; requirement : Light = (); }
	requirement def EmptyAndOwn { subject v : Veh; requirement s[*] : Heavy = (); require constraint { v.mass < 10.0 } }
	requirement def OneFails { subject v : Veh; requirement s[0..1] : Light = new Light(); }
	requirement def OneHolds { subject v : Veh; requirement s[0..1] : Heavy = new Heavy(); }
	requirement def OneWithBody { subject v : Veh; requirement s[0..1] : Heavy = new Heavy() { require constraint { v.mass > 100.0 } } }
	requirement def ManyHold { subject v : Veh; requirement s[*] : Limit = (new Limit(max = 30.0), new Limit(max = 40.0)); }
	requirement def ManyOneFails { subject v : Veh; requirement s[*] : Limit = (new Limit(max = 30.0), new Limit(max = 10.0)); }
	requirement def ManyOfASubtype { subject v : Veh; requirement s[*] : Heavy = (new Heavy(), new Tight()); }
	part box {
		requirement empty : Empty { subject v = veh; }
		requirement nul : Null { subject v = veh; }
		requirement unnamed : Unnamed { subject v = veh; }
		requirement emptyAndOwn : EmptyAndOwn { subject v = veh; }
		requirement oneFails : OneFails { subject v = veh; }
		requirement oneHolds : OneHolds { subject v = veh; }
		requirement oneWithBody : OneWithBody { subject v = veh; }
		requirement manyHold : ManyHold { subject v = veh; }
		requirement manyOneFails : ManyOneFails { subject v = veh; }
		requirement manyOfASubtype : ManyOfASubtype { subject v = veh; }
	}
	attribute emptyResult = box.empty.result;
	attribute emptySubrequirements = size(box.empty.subrequirements);
	attribute emptyConstraints = size(box.empty.constraints);
	attribute nullResult = box.nul.result;
	attribute nullSubrequirements = size(box.nul.subrequirements);
	attribute unnamedResult = box.unnamed.result;
	attribute unnamedSubrequirements = size(box.unnamed.subrequirements);
	attribute emptyAndOwnResult = box.emptyAndOwn.result;
	attribute oneFailsResult = box.oneFails.result;
	attribute oneFailsSubrequirements = size(box.oneFails.subrequirements);
	attribute oneHoldsResult = box.oneHolds.result;
	attribute oneWithBodyResult = box.oneWithBody.result;
	attribute manyHoldResult = box.manyHold.result;
	attribute manyHoldSubrequirements = size(box.manyHold.subrequirements);
	attribute manyOneFailsResult = box.manyOneFails.result;
	attribute manyOneFailsSubrequirements = size(box.manyOneFails.subrequirements);
	attribute manyOfASubtypeResult = box.manyOfASubtype.result;
	calc probe { return one = 1; }
}
