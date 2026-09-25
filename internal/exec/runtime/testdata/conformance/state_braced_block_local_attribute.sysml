package Test {
    // A braced `entry { … }`, `do { … }`, `exit { … }` or transition `do { … }`
    // is one anonymous action, so an attribute declared inside it is local to
    // the block: each block declares its own `k`, shadowing the machine's, and
    // the machine's `k` is what the blocks leave untouched. Each digit records
    // one statement: the entry loop (3, 2, 1), the do block (5), the exit block
    // (7), then the transition's effect (9).
    private import ScalarValues::*;

    state def Blocks {
        attribute log : Integer = 0;
        attribute k : Integer = 100;

        entry; then start;
        state start;

        state working {
            entry {
                attribute k : Integer = 3;
                while k > 0 {
                    assign log := log * 10 + k;
                    assign k := k - 1;
                }
            }
            do {
                attribute k : Integer = 5;
                assign log := log * 10 + k;
            }
            exit {
                attribute k : Integer = 7;
                assign log := log * 10 + k;
            }
        }

        state finished;

        succession first start then working;
        transition first working do {
            attribute k : Integer = 9;
            assign log := log * 10 + k;
        } then finished;
    }
}
