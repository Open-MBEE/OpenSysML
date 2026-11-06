// A state usage parameter's own value takes precedence over the default of the
// parameter it redefines.
package test {
    private import ScalarValues::*;
    state def M {
        in n : Integer default 1;
        entry; then s1;
        state s1;
        state s2;
        transition first s1 if n > 2 then s2;
    }
    state m : M {
        in n = 3;
    }
}
