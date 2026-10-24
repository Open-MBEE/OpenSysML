// An analysis case a part definition owns is a feature of the definition's
// objects the same way, whatever its body reads: with none instantiated the run
// is refused asking for one, rather than performed as a feature of nothing.
package test {
	private import ScalarValues::Real;

	part def Holder {
		attribute weight : Real;
		analysis def Weigh { out k : Real; }
		analysis weigh : Weigh { out :>> k = 1.0; }
	}
}
