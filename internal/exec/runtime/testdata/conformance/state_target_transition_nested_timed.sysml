// A transition written without a source inside a composite state leaves the
// substate declared before it (SysML v2 §7.18.3), not the composite state: the
// timer fires once from coasting, decelerating is entered once and its entry
// action runs once. Were descent the source, the transition would fire again
// from decelerating every 5 s, re-running the entry action.
package test {
    private import SI::*;
    private import ScalarValues::*;

    part def Lander {
        attribute entries : Integer = 0;
        exhibit state flightMode {
            entry; then descent;
            state descent {
                entry; then coasting;
                state coasting;
                accept after 5 [s] then decelerating;
                state decelerating {
                    entry assign entries := entries + 1;
                }
            }
        }
    }

    part scout : Lander;
}
