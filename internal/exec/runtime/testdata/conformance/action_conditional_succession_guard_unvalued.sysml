// A conditional succession whose guard reads a feature carrying no value has no
// Boolean result, so it does not assert the succession (SysML v2 §7.17.5): the
// else succession is taken, as it is when the guard is false.
package test {
    private import ScalarValues::*;
    action chooser {
        attribute v : Real;
        attribute taken : Integer = 0;
        first start;
        then a;
        action a;
        first a if v > 1.0 then b;
        else c;
        action b { assign taken := 1; }
        action c { assign taken := 2; }
    }
}
