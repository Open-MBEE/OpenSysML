// A definition that redefines `this` with a type (`ref part :>> this : H;`)
// reads its performer's features through it: the redefinition is valued from
// the same context occurrence the member read `.this` answers (Objects.kerml
// `ownedPerformances::this default that`), type-checked against H.
package test {
	private import ScalarValues::*;
	part def H {
		attribute level : Integer = 10;
		action def Nudge {
			ref part :>> this : H;
			attribute seen : Integer = this.level;
		}
		action n : Nudge;
	}
	part h : H;
}
