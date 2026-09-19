package Test {
    private import ScalarValues::*;

    // A state's inline do action body stating `first start … then done` is the
    // token flow a standalone action body is: a runs before b, and reaching
    // `done` completes the behavior, so the state completes and the machine
    // takes the succession out of it.
    state def Machine {
        attribute seq : Integer = 0;

        entry; then working;
        state working {
            do action ops {
                first start;
                then action a { assign seq := seq * 10 + 1; }
                then action b { assign seq := seq * 10 + 2; }
                then done;
            }
        }
        succession first working then done;
    }
}
