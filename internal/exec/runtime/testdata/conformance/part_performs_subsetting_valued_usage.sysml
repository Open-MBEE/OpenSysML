// A part's performed action typed by the definition and subsetting a valued
// usage performs through that usage: the object's performance computes from
// the usage's `in` value.
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
        perform action pt : Step :> t;
    }
    part p : P;
}
