// A transition written without a source (SysML v2 §7.18.3, TargetTransitionUsage)
// is a member of the body its source state is declared in: at the top level of a
// state definition or an exhibited state usage, inside a composite state, and
// inside an orthogonal region. Its source is nil in the tree; resolution and
// lowering take the state declared before it, looking past other such transitions.
package Placements {
    private import SI::*;
    attribute def Go;
    attribute def Stop;
    port def Bus;
    action def Log;

    state def Flight {
        attribute alt : ScalarValues::Real = 10.0;
        attribute t : Time::TimeInstantValue;
        port bus : Bus;

        entry; then coasting;
        state coasting;
        accept after 5 [s] then descent.braking;
        accept Go via bus if alt < 20.0 do action note : Log then descent;
        accept at t then landed;
        state descent {
            entry; then gliding;
            state gliding;
            if alt < 5.0 then braking;
            transition if alt >= 5.0 do action log : Log then gliding;
            state braking;
            transition then gliding;
        }
        state landed;
        accept when alt <= 0.0 then landed;
    }

    part def Lander {
        attribute ready : ScalarValues::Boolean = false;
        exhibit state modes parallel {
            state motion {
                entry; then still;
                state still;
                accept Go then moving;
                accept Stop then still;
                state moving;
            }
            state power {
                entry; then off;
                state off;
                if ready then on;
                state on;
            }
        }
    }
}
