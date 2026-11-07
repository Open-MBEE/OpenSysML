// The same through an explicit `redefines`: the redefining parameter's value is
// the machine's, in place of the definition's unvalued one.
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
        in n redefines n = 3;
    }
}
