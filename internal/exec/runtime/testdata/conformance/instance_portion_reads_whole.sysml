// A snapshot or timeslice usage nested in an occurrence is a portion of it
// (SysML v2 §7.9.3; `Occurrences.kerml` `snapshots`/`timeSlices` are portion
// features), the same thing occurring (KerML 1.0 §7.4.5), so a feature the
// portion's own body does not value reads the whole's: its bound value beats
// the type's `default`, while a binding the type states (`product = width *
// height`) is evaluated over the portion's own values.
package test {
    private import ScalarValues::*;
    part def Panel {
        attribute width : Natural;
        attribute height : Natural;
        attribute depth : Natural default 7;
        attribute product : Natural = width * height;
    }
    part def Other { attribute width : Natural = 100; }
    part panel : Panel {
        attribute :>> width = 3;
        attribute :>> height default = 2;
        attribute :>> depth = 4;
        snapshot atReview : Panel { attribute :>> height = 5; }
        snapshot plain : Panel;
        timeslice during : Panel { attribute :>> height = 9; }
        snapshot other : Other;
        assert constraint ownProduct { product == 6 }
        assert constraint snapshotWidth { atReview.width == 3 }
        assert constraint snapshotHeight { atReview.height == 5 }
        assert constraint snapshotDepth { atReview.depth == 4 }
        assert constraint snapshotProduct { atReview.product == 15 }
        assert constraint plainProduct { plain.product == 6 }
        assert constraint plainDepth { plain.depth == 4 }
        assert constraint timesliceProduct { during.product == 27 }
        assert constraint otherWidth { other.width == 100 }
    }
}
