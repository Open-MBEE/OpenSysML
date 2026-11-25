package CheckDerived {
    private import ScalarValues::*;

    // y is derived (y = 2x), so y > 10 and x < 1 cannot both hold: the defining
    // equation is asserted on a constraint as it is on a part.
    constraint k {
        attribute x : Real;
        attribute y : Real = x * 2.0;
        y > 10.0 and x < 1.0
    }
}
