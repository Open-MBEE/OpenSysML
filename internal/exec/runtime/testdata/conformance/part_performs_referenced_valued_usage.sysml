// `perform t;` on a part definition performs the valued usage on each object,
// computing from the usage's `in` value.
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
    part def P {
        perform t;
    }
    part p : P;
}
