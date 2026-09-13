// A lunar mission as behavior: an action flow that spends a delta-v budget and
// skips the landing when it runs short, and the phases as a clocked state machine.
package MissionSequence {
    private import ScalarValues::*;
    private import ISQ::*;
    private import SI::*;

    // Three phases with no order between them: well-formed, but asked to
    // perform it the runtime has nowhere to begin.
    action def LunarMissionUnordered {
        action outbound;
        action lunarOps;
        action returnJourney;
    }

    // The phases in order, each burn drawing on one budget. A decision after
    // lunar orbit insertion checks what is left before committing to a landing.
    action def LunarMission {
        in budget :> ISQ::speed;
        attribute remaining :> ISQ::speed = budget;
        out landed : Boolean = false;
        out returned : Boolean = false;

        first start;
        then action translunarInjection assign remaining := remaining - 3150 ['m/s'];
        then action lunarOrbitInsertion assign remaining := remaining - 900 ['m/s'];
        then decide landingGo;
            if remaining >= 3970 ['m/s'] then descent;
            else transEarthInjection;

        action descent assign remaining := remaining - 2100 ['m/s'];
        then action surfaceOperations assign landed := true;
        then action ascent assign remaining := remaining - 1870 ['m/s'];
        then transEarthInjection;

        action transEarthInjection assign remaining := remaining - 1000 ['m/s'];
        then action splashdown assign returned := true;
        then done;
    }

    // An object performs the flight with its own budget; the runtime runs the
    // action on the object, reading the binding from it.
    part def Flight {
        attribute budget :> ISQ::speed;
        perform action mission : LunarMission { in budget = Flight::budget; }
    }
    part apollo11 : Flight { attribute :>> budget = 9500 ['m/s']; }
    part apollo8 : Flight { attribute :>> budget = 5500 ['m/s']; }

    // The phases as modes on the mission clock, in seconds; the executor fires
    // the transitions as they fall due.
    state def MissionPhases {
        entry; then prelaunch;

        state prelaunch;
        state translunarCoast;
        state lunarOrbit;
        state onSurface;
        state transEarthCoast;
        state recovered;

        transition first prelaunch accept after 9000 [s] then translunarCoast;
        transition first translunarCoast accept after 270000 [s] then lunarOrbit;
        transition first lunarOrbit accept after 100000 [s] then onSurface;
        transition first onSurface accept after 78000 [s] then transEarthCoast;
        transition first transEarthCoast accept after 245000 [s] then recovered;
        transition first recovered then done;
    }

    part def Mission {
        exhibit state phases : MissionPhases;
    }

    part mission : Mission;
}
