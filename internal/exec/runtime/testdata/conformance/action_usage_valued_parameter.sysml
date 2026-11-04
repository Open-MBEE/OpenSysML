// An action usage's own `in` value redefines the parameter it is named for
// (SysML v2.1 §7.17.2): performed with no argument, the usage computes from it.
package test {
    private import ScalarValues::*;
    action def Step {
        in x : Integer;
        out y : Integer;
        first start;
        then assign y := x + 1;
    }
    action s : Step {
        in x = 9;
    }
}
