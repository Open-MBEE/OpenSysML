// A transition guard reading a feature that carries no value has no Boolean
// result, so the transition is not enabled (SysML v2 §7.18.2) and the machine
// rests in the source state, running, rather than failing to start.
package test {
    private import ScalarValues::*;
    part def Ctl {
        attribute v : Real;
        exhibit state sm {
            entry; then s1;
            state s1;
            transition first s1 if v > 1.0 then s2;
            state s2;
        }
    }
    part ctl : Ctl;
}
