// A metadata annotation written among an analysis definition's body members,
// before its subject, annotates the case and is no step of it: the result
// expression runs as it does with the annotation stated from outside.
package test {
	private import ScalarValues::*;

	metadata def Credibility { attribute level : Integer; }
	part def Thing { attribute v : Real; }
	part thing : Thing { attribute :>> v = 3.0; }

	analysis def Weigh {
		@Credibility { level = 3; }
		subject t : Thing;
		return mass : Real = t.v * 2.0;
	}
	analysis weigh : Weigh { subject t = thing; }
}
