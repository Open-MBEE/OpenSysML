// An unguarded entry transition among guarded ones is the alternative whose
// guard always holds: declared last it is the default the machine starts in
// when no earlier guard holds (SysML v2 §7.18.3, `entry; if c then s; then t;`).
package test {
    private import ScalarValues::*;

    state def Heater {
        attribute cold : Boolean = false;
        attribute readyEntries : Integer = 0;

        entry;
        if cold then warming;
        then ready;

        state warming;
        state ready {
            entry assign readyEntries := readyEntries + 1;
        }
    }
}
