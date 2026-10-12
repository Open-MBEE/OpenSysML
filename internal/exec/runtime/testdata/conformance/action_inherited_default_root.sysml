package test {
    private import ScalarValues::*;

    // The unsupplied `in z = x * 2` parameter binds z to the expression for the
    // performance, so it follows the value assigned to x later in the body.
    action def Base {
        in x : Integer = 3;
        in z : Integer = x * 2;
        out y : Integer;
        out r : Integer;
    }

    action def Derived :> Base {
        first start;
        then assign y := x + 1;
        then assign x := 10;
        then assign r := z;
        then done;
    }
}
