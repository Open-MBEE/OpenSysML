// A usage parameter's own value takes precedence over the default of the
// parameter it redefines.
package test {
    private import ScalarValues::*;
    action def Step {
        in x : Integer default 1;
        out y : Integer;
        first start;
        then assign y := x + 1;
    }
    action s : Step {
        in x = 9;
    }
}
