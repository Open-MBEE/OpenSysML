// An ordered feature is unique unless declared nonunique, so its values are
// held in the order written with no repeat among them: the values flow through
// ordered unique features and an OrderedSet as written — never sorted, never
// dropped — and a List holds the repeat a unique feature would refuse. The
// trace is the golden for that order.
package test {
	private import ScalarValues::*;
	private import Collections::*;
	private import ControlFunctions::*;

	attribute xs : Integer[*] ordered = (30, 10, 20);
	attribute os : OrderedSet { :>> elements = (xs#(3), xs#(1), xs#(2)); }
	attribute list : List { :>> elements = (xs#(1), xs#(1), xs#(2)); }

	calc walk {
		attribute scaled : Integer[*] ordered = xs->collect{in x; x - 5};
		attribute fromSet : Integer[*] ordered = os.elements;
		return : Integer[*] ordered nonunique = (xs#(1), xs#(2), xs#(3), fromSet#(1), fromSet#(2), fromSet#(3), scaled#(1), scaled#(3), list.elements#(2));
	}
}
