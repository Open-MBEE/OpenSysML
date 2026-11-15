package BoundLiteral {
    private import ScalarValues::*;

    // limit is bound with `=`, so the feature holds 1.0 at all times
    // (KerML 1.1 §7.3.4.5) and no x is both above it and below zero.
    constraint bound {
        attribute limit : Real = 1.0;
        attribute x : Real;
        x > limit and x < 0.0
    }
}
