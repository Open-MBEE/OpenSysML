package P {
    private import ScalarValues::*;
    calc def Scale {
        in <xs> x : Integer;
        inout <ks> k : Integer = 2;
        out <ys> y : Integer = x * k;
        return <r> result : Integer = y;
    }
    action def Move { in <ds> d : Integer; out <es> e : Integer; }
}
