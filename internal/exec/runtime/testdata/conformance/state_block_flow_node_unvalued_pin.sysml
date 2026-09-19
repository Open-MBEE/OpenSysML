// An action node in a state's body holds the pin it declares without a value
// (`out v : Integer;`) as its own: the node's assignment to `v` writes that pin,
// not the machine's same-named attribute, and a sibling node's `v` is another.
// The machine's `v` stays 100 only if neither node's assignment escaped its frame.
package test {
    private import ScalarValues::*;

    state def Tally {
        attribute total : Integer = 0;
        attribute v : Integer = 100;

        entry; then init;
        state init;

        state active {
            entry action {
                if total == 0 {
                    action p {
                        out v : Integer;
                        assign v := 1;
                        assign total := total + v;
                    }
                    action q {
                        out v : Integer;
                        assign v := 20;
                        assign total := total + v;
                    }
                }
            }
        }

        succession first init then active;
        succession first active then done;
    }
}
