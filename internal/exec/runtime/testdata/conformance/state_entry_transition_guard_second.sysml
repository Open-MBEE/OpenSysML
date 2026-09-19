// The first entry transition's guard is false, so the second chooses the start
// (SysML v2 §7.18.3): the machine starts in `on`, whose entry action runs once,
// and `off` is never entered.
package test {
    private import ScalarValues::*;

    state def Heater {
        attribute isInitOff : Boolean = false;
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
