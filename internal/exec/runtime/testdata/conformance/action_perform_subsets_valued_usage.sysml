// A performed action typed by the definition and subsetting a valued usage of it
// is a performance of that usage (KerML 1.1 §7.3.4.3): it computes from the
// usage's `in` value rather than the definition's unvalued parameter.
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
