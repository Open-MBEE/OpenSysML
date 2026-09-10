package Test {
    private import ScalarValues::*;
    private import SI::*;

    // An `inout` pin of a do behavior's typed action usage may be valued by a
    // literal as well as by a feature: `mode` starts as `Mode::idle` and is the
    // pin's own value, so what the performance leaves in it is not written back
    // to the literal, while `n`, valued by the feature `ticks`, writes back to
    // it when the performance ends. `ticks` counts from the mode the pin held.
    enum def Mode { idle; busy; }

    state def Machine {
        action def Poll {
            inout n : Integer;
            inout mode : Mode;
            first start;
            then action wait accept after 3 [s];
            then action count assign n := if mode == Mode::idle ? n + 1 else n + 100;
            then action flip assign mode := Mode::busy;
            then done;
        }

        attribute ticks : Integer = 5;

        entry; then watching;
        state watching {
            do action poll : Poll { inout n = ticks; inout mode = Mode::idle; }
        }
        succession first watching then finished;
        state finished;
    }
}
