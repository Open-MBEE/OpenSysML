// A requirement's subject is its first parameter, and one declaring a default may
// be left unbound by a call: the call binds the rest by name and the subject takes
// its default, so the candidate whose subject is defaulted is the one selected
// among same-named ones. A usage redefining the subject without restating a value
// inherits the default.
package test {
	private import ScalarValues::*;
	part def Widget { attribute m : Real; }
	part light : Widget { attribute :>> m = 1.0; }
	part heavy : Widget { attribute :>> m = 9.0; }
	package A {
		requirement def Light {
			subject w : Widget default = light;
			in limit : Real default = 5.0;
			require constraint { w.m < limit }
		}
	}
	package B {
		requirement def Light {
			subject w : Widget;
			in limit : Real;
			in slack : Real;
			require constraint { w.m < limit + slack }
		}
	}
	private import A::*;
	private import B::*;
	requirement lightEnough : A::Light { subject :>> w; in :>> limit = 0.5; }
	calc picks {
		attribute defaulted : Boolean = Light(limit = 2.0);
		attribute redefined : Boolean = lightEnough();
		attribute given : Boolean = Light(heavy, 2.0, 1.0);
		return : Boolean = defaulted and not redefined and not given;
	}
}
