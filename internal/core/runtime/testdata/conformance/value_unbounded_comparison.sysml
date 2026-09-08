// `*` (KerML 8.3.3.1 LiteralInfinity) is a value of its own: it exceeds every
// finite number, whatever its type, and equals itself.
package test {
    private import ScalarValues::*;

    part def Bounds {
        attribute unbounded = *;
        attribute exceedsNatural = * > 1000000;
        attribute exceedsInteger = * > -3;
        attribute exceedsReal = * > 1.5e10;
        attribute belowUnbounded = 42 < *;
        attribute equalsItself = * == *;
        attribute atMostItself = * <= *;
        attribute aboveItself = * > *;
        attribute atMostFinite = * <= 7;
        attribute differsFromFinite = * != 7;
    }
}
