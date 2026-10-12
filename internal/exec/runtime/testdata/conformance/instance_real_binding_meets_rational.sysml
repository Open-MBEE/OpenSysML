// A Real feature bound to a Rational holds its nearest binary64, and a Rational
// meets a binary64 Real at Real precision, as mixed arithmetic does, so the
// binding's equality holds (KerML 1.0 §7.4.9). Rationals compare with each other
// exactly, an Integer with a Real exactly, and a Rational past the binary64
// range compares as an infinity.
package test {
    private import ScalarValues::*;

    part def Sample {
        attribute x : Real = 0.1;
        attribute y : Real = 1 / 3;
        attribute big : Real = 1.0e308;
        assert constraint boundTenth { x == 0.1 }
        assert constraint boundThird { y == 1 / 3 }
        assert constraint tenthNeitherAboveNorBelow { not (x > 0.1) and not (x < 0.1) and x <= 0.1 and x >= 0.1 and not (x != 0.1) }
        assert constraint thirdAtRealPrecision { RationalFunctions::rat(1, 3) == 1.0 / 3.0 and RationalFunctions::rat(1, 3) == RealFunctions::ToReal("1") / 3.0 }
        assert constraint rationalsExact { RationalFunctions::rat(1, 3) != RationalFunctions::rat(6004799503160661, 18014398509481984) }
        assert constraint beyondRange { 1.0e400 > big and 1.0e400 != big and -1.0e400 < -big }
        assert constraint integerExact { 9007199254740993 > RealFunctions::ToReal("9007199254740992") }
    }

    part sample : Sample;
}
