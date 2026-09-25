// A feature's values are instances of its type: untyped members listed as the
// value of `edges : Segment` become Segments, carrying its features, while a
// value no type of the member is comparable with stays refused.
package test {
    private import ScalarValues::*;

    item def Segment {
        attribute span : Real;
        item ends [2];
    }

    item def Loop {
        item edges : Segment [*];
    }

    item def Square :> Loop {
        item :>> edges [4] = (e1, e2, e3, e4);
        item e1 [1];
        item e2 [1];
        item e3 [1];
        item e4 [1];
    }

    item def Broken :> Loop {
        item :>> edges [1] = (count);
        attribute count : Integer = 3;
    }

    part def Sheet {
        item sq : Square;
        item broken : Broken;
    }
}
