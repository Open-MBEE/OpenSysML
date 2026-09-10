// Guarded entry transitions (SysML v2 §7.18.3 `EntryTransitionMember`) choose
// the state the machine starts in: the first, in declaration order, whose
// guard holds. Here the first holds, so the machine starts in `off` and only
// its entry action runs.
package test {
    private import ScalarValues::*;

    state def Heater {
        attribute isInitOff : Boolean = true;
        attribute offEntries : Integer = 0;
        attribute onEntries : Integer = 0;

        entry;
        if isInitOff then off;
        if not isInitOff then on;

        state off {
            entry assign offEntries := offEntries + 1;
        }
        state on {
            entry assign onEntries := onEntries + 1;
        }
    }
}
