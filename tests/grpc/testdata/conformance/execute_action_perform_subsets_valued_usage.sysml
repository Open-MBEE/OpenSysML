package test {
    private import ScalarValues::*;
    action def Step {
        in x : Integer;
        out y : Integer;
        first start;
        then assign y := x + 1;
    }
    action t : Step {
        in x = 9;
    }
    action def Outer {
        out r : Integer;
        first start;
        then perform action pt : Step :> t;
        then assign r := pt.y;
    }
    action o : Outer;
}
