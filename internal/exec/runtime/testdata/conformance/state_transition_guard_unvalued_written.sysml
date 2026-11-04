// Once the feature a guard reads is written, the guard is read again where a
// false guard would be, at the completion of the re-entered source state, and
// the transition fires.
package test {
    private import ScalarValues::*;
    attribute def Set;
    part def Ctl {
        attribute v : Real;
        exhibit state sm {
            entry; then s1;
            state s1;
            transition first s1 if v > 1.0 then s2;
            transition first s1 accept Set do assign v := 2.0 then s1;
            state s2;
        }
    }
    part ctl : Ctl;
}
