// A pin-only typed node in a loop of a state's entry body is a performance of
// the action it names: its input binds from the loop variable and its `out` is
// read after the node, because a typed node's own statements would be unordered
// with the callee's steps (see the oracle section).
package test {
    private import ScalarValues::*;

    action def Scale {
        in x : Integer;
        in factor : Integer = 10;
        out y : Integer;

        first start;
        then action scaling { assign y := x * factor; }
        then done;
    }

    state def Tally {
        attribute total : Integer = 0;
        attribute runs : Integer = 0;

        entry; then init;
        state init;

        state active {
            entry action {
                for i in 1..3 {
                    action scaled : Scale {
                        in x = i;
                    }
                    assign total := total + scaled.y;
                    assign runs := runs + 1;
                }
                then if total > 50 {
                    action doubled = Scale(total, 2) {
                        assign total := y;
                    }
                }
            }
        }

        succession first init then active;
        succession first active then done;
    }
}
