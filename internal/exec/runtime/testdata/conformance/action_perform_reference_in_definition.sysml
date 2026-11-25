// `perform t;` in an action definition's body performs the valued usage, so a
// performance of the definition computes from the usage's `in` value.
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
        then perform t;
        then assign r := t.y;
    }
    action o : Outer;
}
