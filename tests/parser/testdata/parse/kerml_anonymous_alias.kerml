// An alias member need not be named (KerML.xtext AliasMember: memberName is optional).
package P {
	class M [0..1] {
		alias for Base::exactlyOne;
	}
	alias <x> for P::M;
	private alias for P::M {
		doc /* Anonymous, with a body. */
	}
}
