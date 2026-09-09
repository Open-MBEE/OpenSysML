// Guarded entry transitions choose the start at every depth (SysML v2 §7.18.3):
// the machine's own body, a composite state's body, and each orthogonal region
// of a parallel state, each evaluated when that body is entered. The guards
// read attributes an earlier entry action assigned, so evaluation order is
// observable: `mode` is 2 by the time `running` is entered, so its entry
// transitions choose `fast`, and the regions of `fast` choose `b` and `d`.
package test {
    private import ScalarValues::*;

    part def Controller {
        attribute mode : Integer = 1;
        attribute slowEntries : Integer = 0;
        attribute fastEntries : Integer = 0;

        exhibit state control {
            entry assign mode := mode + 1;
            if mode == 1 then idle;
            if mode == 2 then running;

            state idle;
            state running {
                entry;
                if mode < 2 then slow;
                if mode >= 2 then fast;

                state slow {
                    entry assign slowEntries := slowEntries + 1;
                }
                state fast parallel {
                    entry assign fastEntries := fastEntries + 1;
                    state left {
                        entry;
                        if mode == 1 then a;
                        then b;
                        state a;
                        state b;
                    }
                    state right {
                        entry;
                        if mode == 2 then d;
                        then c;
                        state c;
                        state d;
                    }
                }
            }
        }
    }

    part unit : Controller;
}
