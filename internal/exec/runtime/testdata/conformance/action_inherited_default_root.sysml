package test {
    private import ScalarValues::*;

    // The root performance seeds the defaults inherited from the general action
    // when it starts, as it seeds its own: z holds x * 2 as x was then, not as
    // the body later reassigns it.
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
