// A state usage's own `in` value redefines the parameter it is named for
// (KerML 1.1 §8.4.4.7.2): run with no argument, the guard reads it.
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
}
