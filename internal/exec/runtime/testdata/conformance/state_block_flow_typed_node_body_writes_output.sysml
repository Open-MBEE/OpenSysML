// A typed node in a state's body returns its `out` values once its own body has
// run: `scaled` performs Scale (y = 30), its body adds one, and the state's `y`
// reads 31.
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

    state def Adjust {
        attribute y : Integer = 0;
        attribute seen : Integer = 0;

        entry; then init;
        state init;

        state active {
            entry action {
                if seen == 0 {
                    action scaled : Scale {
                        in x = 3;
                        assign y := y + 1;
                    }
                    assign seen := y;
                }
            }
        }

        succession first init then active;
        succession first active then done;
    }
}
