// A usage performed as a step of another action's body computes from its own
// `in` value as it does performed on its own.
package test {
    private import ScalarValues::*;
    action def Step {
        in x : Integer;
        out y : Integer;
        first start;
        then assign y := x + 1;
    }
    action s : Step {
        in x = 9;
    }
    action caller {
        out r : Integer;
        first start;
        then perform s;
        then assign r := s.y;
    }
}
