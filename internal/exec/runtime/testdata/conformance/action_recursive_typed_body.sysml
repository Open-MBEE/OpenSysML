package test {
    private import ScalarValues::*;

    // A typed action body nested in its own definition: each level of the
    // recursion performs Walk's merged body again, one level deeper, until
    // the guard holds. Lowering unfolds Walk once; the executor runs it per
    // level and reports the depth the deepest level reached.
    action def Walk {
        in depth : Integer;
        out reached : Integer = 0;
        attribute visited : Integer = 1;
        if depth > 0 {
            action step : Walk { in depth = depth - 1; assign visited := visited + 1; }
            then assign reached := step.reached + 1;
        } else {
            assign reached := 1;
        }
    }

    action run {
        attribute total : Integer = 0;
        action w : Walk { in depth = 3; }
        then assign total := w.reached;
    }
}
