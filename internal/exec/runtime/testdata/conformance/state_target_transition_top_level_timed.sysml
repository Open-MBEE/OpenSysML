// A transition written without a source leaves the state declared before it in
// the same body (SysML v2 §7.18.3, TargetTransitionUsage), here at the top level
// of an exhibited state machine: `accept after 5 [s]` leaves coasting, so the
// timer arms once and the lander ends up decelerating with braking set.
package test {
    private import SI::*;
    private import ScalarValues::*;

    part def Lander {
        attribute braking : Boolean = false;
        exhibit state flightMode {
            entry; then coasting;
            state coasting;
            accept after 5 [s] then decelerating;
            state decelerating {
                entry assign braking := true;
            }
        }
    }

    part scout : Lander;
}
