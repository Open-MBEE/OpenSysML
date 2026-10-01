// A guard on a segment out of a junction reads a call trigger's argument by the
// accepting transition's name, `set.value`, as its effect does: the junction is
// resolved within the performance of the transition whose trigger bound it.
package StateJunctionGuardReadsCallArgument {
    private import ScalarValues::*;

    state Controller {
        attribute speed : Integer = 0;

        entry; then idle;
        state idle;
        junction route;
        state cruising;
        state stopped;

        transition set first idle accept setSpeed(value) then route;
        transition go first route if set.value > 0 do assign speed := set.value then cruising;
        transition halt first route if set.value <= 0 then stopped;
        transition first cruising then done;
        transition first stopped then done;
    }
}
