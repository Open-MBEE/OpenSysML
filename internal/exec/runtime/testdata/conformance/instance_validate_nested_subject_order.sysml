// A satisfaction asserted `by` an object whose nested part carries the requirement
// is about that nested object, and is reported with it: after the holder's own
// verdicts and after the nested object's carried ones, not in the holder's group.
package test {
    private import ScalarValues::Real;

    part def Inner {
        attribute c : Real = 2.0;
        assert constraint positive { c > 0.0 }
        requirement lim {
            require constraint { c < 5.0 }
        }
    }

    part def Outer {
        attribute n : Real = 1.0;
        assert constraint one { n == 1.0 }
        part inner : Inner;
    }

    part big : Outer;
    satisfy Inner::lim by big;
}
