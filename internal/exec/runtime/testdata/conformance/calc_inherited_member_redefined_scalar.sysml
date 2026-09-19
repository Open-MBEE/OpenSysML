// A member inherited as `[0..*]` and redefined `[1]` is one value; redefined
// without a multiplicity it inherits `[0..*]` and, holding one value, denotes it.
package test {
	private import ScalarValues::*;

	attribute def Base { attribute xs : Real[0..*]; }
	attribute def Scalar :> Base { attribute xs : Real[1] :>> Base::xs; }
	attribute def Inherited :> Base { attribute xs :>> Base::xs; }
	attribute def Many :> Base { attribute xs : Real[0..*] :>> Base::xs; }

	calc def Redefined {
		attribute s : Scalar = new Scalar(xs = 5.0);
		attribute i : Inherited = new Inherited(xs = 5.0);
		attribute m : Many = new Many(xs = (5.0, 6.0));
		out fromScalar = s.xs - 1.0;
		out fromInherited = i.xs - 1.0;
		out fromMany : Real[0..*] = m.xs;
	}

	calc c : Redefined;
}
