package P {
	public import A::B::*;
	private import C::D;
	private import E::**;
	namespace N {
		alias X for P::Q;
	}
	dependency from P to Q;
	comment /* a note */
	doc /* the docs */
}
