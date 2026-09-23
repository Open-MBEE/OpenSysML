// A deferred occurrence is dispatched once a state that handles it is active, and
// the guard of the transition it then fires reads its payload through the
// transition's name, `raise.l`, as it does for an occurrence never deferred.
package StateGuardReadsDeferredPayload {
    private import ScalarValues::*;

    attribute def Level :> Integer;

    state Machine {
        attribute chosen : Integer = 0;

        entry; then busy;
        state busy {
            defer Level;
        }
        state ready;
        state high;
        state low;

        transition first busy when Go then ready;
        transition raise first ready accept l : Level if raise.l > 5 do assign chosen := raise.l then high;
        transition drop first ready accept l : Level if raise.l <= 5 then low;
        transition first high then done;
        transition first low then done;
    }
}
