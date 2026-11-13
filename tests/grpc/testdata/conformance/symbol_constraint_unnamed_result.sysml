// A constraint def inherits BooleanEvaluation's unnamed return, which redefines
// Evaluation::result by position and so takes its name (KerML 8.3.3.3): the
// member is listed as result, under that qualified id, not positionally.
package P {
	private import ScalarValues::*;
	constraint def Cons { in x : Real; x > 0 }
}
