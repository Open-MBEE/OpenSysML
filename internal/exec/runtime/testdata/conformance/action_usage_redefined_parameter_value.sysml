// The same through an explicit `redefines`: the redefining parameter's value is
// the performance's, in place of the definition's unvalued one.
package test {
    private import ScalarValues::*;
    action def Step {
        in x : Integer;
        out y : Integer;
        first start;
        then assign y := x + 1;
    }
    action s : Step {
        in x redefines x = 9;
    }
}
