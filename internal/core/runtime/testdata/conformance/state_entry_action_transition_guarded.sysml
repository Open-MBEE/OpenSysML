// Guarded transitions out of a named entry action choose the start the same
// way the anonymous `entry; if c then s;` does (SysML v2 §7.18.3): the first,
// in declaration order, whose guard holds.
package test {
    private import ScalarValues::*;

    state def Heater {
        attribute isInitOff : Boolean = false;
        attribute onEntries : Integer = 0;

        entry action begin { }
        transition begin if isInitOff then off;
        transition begin if not isInitOff then on;

        state off;
        state on {
            entry assign onEntries := onEntries + 1;
        }
    }
}
