// A typed action node declared in a loop or branch of a state's entry body is a
// performance of the action it names: the `in` value it declares binds the
// callee's parameter (a loop variable reaching it), the callee's `out` comes
// back to the node, and the node's own body runs after the callee, reading that
// out and writing the machine's attributes. Arguments bind the same way, so
// total ends at 120 only if every pin, argument and body statement took effect.
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
                        assign total := total + y;
                        assign runs := runs + 1;
                    }
                }
                if total > 50 {
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
