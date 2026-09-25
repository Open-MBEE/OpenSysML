// A guard naming a transition other than the one it guards reads nothing: `lower`
// is not being taken while `raise` is, so `lower.l` is null and comparing it is
// the typed error a null operand raises, rather than a silent false.
package StateGuardNamesTransitionNotTaken {
    private import ScalarValues::*;

    attribute def Level :> Integer;

    state Machine {
        entry; then idle;
        state idle;
        state high;
        state low;

        transition raise first idle accept l : Level if lower.l < 0 then high;
        transition lower first high accept l : Level then low;
        transition first low then done;
    }
}
