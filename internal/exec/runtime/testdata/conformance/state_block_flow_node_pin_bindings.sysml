// A state body's block binds and flows at its action nodes' pins as an action
// body's does: `bind add.a = seen` seeds the node from a state attribute before
// it runs, `flow add.sum to mirror.n` carries its output to a sibling's pin,
// a `bind` at an output returns it to a state attribute (`total`), and a later
// statement reads the nodes' pins as `add.sum` and `mirror.doubled`.
package test {
    private import ScalarValues::*;

    action def Adder {
        in a : Integer;
        in b : Integer;
        out sum : Integer;

        first step;
        action step { assign sum := a + b; }
    }

    state def Pins {
        attribute seen : Integer = 5;
        attribute total : Integer = 0;
        attribute doubled : Integer = 0;
        attribute checked : Integer = 0;

        entry; then init;
        state init;

        state active {
            entry action {
                if seen > 0 {
                    bind add.a = seen;
                    action add : Adder { in b = 6; }
                    bind add.sum = total;
                    flow add.sum to mirror.n;
                    action mirror {
                        in n : Integer;
                        out d : Integer;
                        assign d := n * 2;
                    }
                    assign doubled := mirror.d;
                    assign checked := add.sum * 100 + mirror.n;
                }
            }
        }

        succession first init then active;
        succession first active then done;
    }
}
