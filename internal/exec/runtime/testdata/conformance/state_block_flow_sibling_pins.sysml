// Two action nodes of a state body's loop declare same-named `out v` pins, each in a
// frame of its own: a sibling sums them as `p.v + q.v`, once per iteration, and the
// pins of one iteration do not leak into the next (`q` reads `p.v` afresh each time).
package test {
    private import ScalarValues::*;

    state def Sums {
        attribute total : Integer = 0;
        attribute i : Integer = 0;

        entry; then init;
        state init;

        state summing {
            entry action {
                while i < 3 {
                    action p { out v : Integer; assign v := i + 1; }
                    action q { out v : Integer; assign v := p.v * 10; }
                    action fin { assign total := total + p.v + q.v; }
                    assign i := i + 1;
                }
            }
        }

        succession first init then summing;
        succession first summing then done;
    }
}
