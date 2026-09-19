// A satisfaction assertion of a requirement declared on a nested definition,
// asserted `by` the object holding it: the requirement's condition reads the
// nested object's redefined value rather than the declared default, so the
// verdict agrees with the one `%requirement` gives for the same object.
package test {
    private import ScalarValues::Real;

    part def Inner {
        attribute c : Real = 1.0;
        requirement lim {
            require constraint { c < 10.0 }
        }
    }

    part def Outer {
        part b : Inner;
    }

    part low : Outer {
        part :>> b {
            attribute :>> c = 2.0;
        }
    }

    part high : Outer {
        part :>> b {
            attribute :>> c = 99.0;
        }
    }

    part analysisContext {
        assert satisfy Inner::lim by low;
        assert satisfy Inner::lim by high;
    }
}
