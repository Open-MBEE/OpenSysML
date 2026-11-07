package test {
    private import ScalarValues::*;
    state def M {
        in n : Integer;
        entry; then s1;
        state s1;
        state s2;
        transition first s1 if n > 2 then s2;
    }
    state m : M {
        in n = 3;
    }
    part def P {
        exhibit state es : M :> m;
    }
    part p : P;
}
