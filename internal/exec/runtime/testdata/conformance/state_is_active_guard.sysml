package test {
    private import ScalarValues::*;
    private import StateActivity::*;

    // Guards read activity: of two transitions on one trigger, the one guarded
    // on the current state's own activity fires and the one guarded on an
    // inactive state does not, including `not` and `==` forms.
    item def Go;

    state def Guarded {
        entry; then a;
        state a;
        state wrong;
        state right;
        transition first a accept Go if c.isActive then wrong;
        transition first a accept Go if a.isActive == true and not c.isActive then right;
        state c;
    }
}
