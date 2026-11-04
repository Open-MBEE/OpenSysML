// A transition guard that fails to evaluate on its own terms is still an
// error: the machine fails to start, recorded on the object, which stands.
package test {
    private import ScalarValues::*;
    part def Ctl {
        attribute z : Real = 0.0;
        exhibit state sm {
            entry; then s1;
            state s1;
            transition first s1 if 1.0 / z > 1.0 then s2;
            state s2;
        }
    }
    part ctl : Ctl;
}
