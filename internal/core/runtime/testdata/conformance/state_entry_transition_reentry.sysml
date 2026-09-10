// A composite state's entry transitions are tried each time the state is
// entered, not only when the machine starts (SysML v2 §7.18.3): the first entry
// into `running` finds `visits == 1` and starts in `slow`; re-entering it after
// `paused` finds `visits == 2`, so the unguarded alternative starts `fast`.
package test {
    private import ScalarValues::*;

    attribute def Pause;
    attribute def Resume;
    attribute def Stop;

    state def Control {
        attribute visits : Integer = 0;
        attribute slowEntries : Integer = 0;
        attribute fastEntries : Integer = 0;

        entry; then running;
        state running {
            entry assign visits := visits + 1;
            if visits == 1 then slow;
            then fast;
            state slow {
                entry assign slowEntries := slowEntries + 1;
            }
            state fast {
                entry assign fastEntries := fastEntries + 1;
            }
        }
        accept Pause then paused;
        state paused;
        accept Resume then running;
        transition first running accept Stop then finished;
        state finished;
    }
}
