// A typed action node in a state's body holds every pin of the action it
// performs, not only those it redeclares: an argument overrides the default the
// node states for `factor` and the body reads the bound value, the callee's
// `x` and `y` are in reach of the body, and an output named like a state
// attribute (`y`) comes back to that attribute as it does to an enclosing
// action's feature, while the node's redeclared `factor` stays its own.
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

    state def Pins {
        attribute y : Integer = 0;
        attribute factor : Integer = 1;
        attribute seen : Integer = 0;

        entry; then init;
        state init;

        state active {
            entry action {
                if seen == 0 {
                    action scaled = Scale(x = 7, factor = 3) {
                        in factor = 100;
                        assign seen := x * 1000 + factor * 10 + y;
                    }
                }
            }
        }

        succession first init then active;
        succession first active then done;
    }
}
