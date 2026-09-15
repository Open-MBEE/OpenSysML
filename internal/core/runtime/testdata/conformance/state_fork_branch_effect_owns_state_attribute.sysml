// A fork branch's effect reads and writes the attributes of the state declaring
// the fork, as a transition leaving one of its substates does: `count` here is
// `outer`'s, not the machine's, and `outer`'s exit sees what the branches wrote.
package ForkBranchEffectOwnsStateAttribute {
    private import ScalarValues::*;

    state Machine {
        attribute seen : Integer = 0;

        entry; then outer;
        state outer {
            attribute count : Integer = 0;

            entry; then init;
            state init;
            state work parallel {
                state left {
                    state a;
                    transition first a then done;
                }
                state right {
                    state b;
                    transition first b then done;
                }
            }
            fork split;

            transition first init then split;
            transition first split do { assign count := count + 1; } then a;
            transition first split do { assign count := count + 10; } then b;
            transition first work then done;
            exit { assign seen := count; }
        }

        transition first outer then done;
    }
}
