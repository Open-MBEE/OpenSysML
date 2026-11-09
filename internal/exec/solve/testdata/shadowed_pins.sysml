package test {
    private import ScalarValues::*;

    part h {
        attribute x : Real default 2.0;
        attribute w : Real = 3.0;

        constraint k {
            attribute x : Real = 1.0;
            x > 1.5 and w > 0.0
        }
    }
}
