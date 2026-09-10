package Test {
    private import ScalarValues::*;

    // An attribute an inline do body declares belongs to the body's own
    // performance, as a standalone action's does: its default is evaluated
    // where it was written, a node's assignment to it is what the next node
    // reads, and it shadows the machine's attribute of the same name without
    // writing it.
    state def Machine {
        attribute seq : Integer = 0;
        attribute total : Integer = 0;
        attribute n : Integer = 100;

        entry; then working;
        state working {
            do action ops {
                attribute n : Integer = seq + 2;
                first start;
                then action a { assign n := n * 10 + 1; }
                then action b { assign total := n; }
                then done;
            }
        }
        succession first working then done;
    }
}
