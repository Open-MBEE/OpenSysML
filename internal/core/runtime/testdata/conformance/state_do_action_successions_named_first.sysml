package Test {
    private import ScalarValues::*;

    // A state's inline do action body whose nodes are ordered by `first a then b`
    // runs a before b: the named node is where the flow starts, and the flow ends
    // when b, having no succession out of it, retires the token.
    state def Machine {
        attribute seq : Integer = 0;

        entry; then working;
        state working {
            do action ops {
                action a { assign seq := seq * 10 + 1; }
                action b { assign seq := seq * 10 + 2; }
                first a then b;
            }
        }
        succession first working then done;
    }
}
