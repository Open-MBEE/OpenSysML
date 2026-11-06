// A state usage parameter stating no value keeps the default of the parameter
// it redefines.
package test {
    private import ScalarValues::*;
    state def M {
        in n : Integer default 3;
        entry; then s1;
        state s1;
        state s2;
        transition first s1 if n > 2 then s2;
    }
    state m : M {
        in n;
    }
}
