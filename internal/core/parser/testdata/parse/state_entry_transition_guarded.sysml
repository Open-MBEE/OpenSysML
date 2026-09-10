// The sourceless shorthand written right after a body's entry action is an entry
// transition (SysML v2 §7.18.3, EntryTransitionMember): guarded alternatives, an
// unguarded one among them, after `entry;`, after an entry assignment, after a
// named entry action, and in a composite state, an orthogonal region and an
// exhibited state. Its source is nil in the tree like any sourceless transition.
package GuardedEntry {
    private import ScalarValues::*;

    state def Thermostat {
        attribute cold : Boolean = true;
        attribute mode : Integer = 0;

        entry assign mode := mode + 1;
        if cold then heating;
        if not cold then idle;
        then off;
        state heating {
            entry action warm { }
            if mode > 1 then fast;
            then slow;
            state fast;
            state slow;
        }
        state idle;
        state off;
    }

    state def Plant parallel {
        attribute cold : Boolean = false;
        state climate {
            entry; if cold then heating;
            then idle;
            state heating;
            state idle;
        }
        state lights {
            entry action boot { }
            transition boot if cold then dim;
            transition boot then bright;
            state dim;
            state bright;
        }
    }

    part def Lander {
        attribute armed : Boolean = false;
        exhibit state flight {
            entry; if armed then descent;
            if not armed then coasting;
            state coasting;
            state descent;
        }
    }
}
