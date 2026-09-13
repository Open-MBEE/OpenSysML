// `all T` over a data type has no finite extent to enumerate — a scalar domain or a
// structured type alike, since a run creates no data values and the value `origin` holds
// is one Point, not the type's extent: it is a typed refusal, not an empty sequence.
package test {
    private import ScalarValues::*;

    attribute def Point {
        attribute x : Real = 1.0;
    }

    part counter {
        attribute origin : Point;
        attribute integers = all Integer;
        attribute strings = all String;
        attribute points = all Point;
    }
}
