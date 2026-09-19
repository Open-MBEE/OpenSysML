package Test {
    private import ScalarValues::*;

    // Inline entry and exit action bodies stating successions run as token flows
    // too: the entry body runs a then b on entering the state, the exit body
    // runs c then d on leaving it, each `then done` completing its behavior.
    state def Machine {
        attribute seq : Integer = 0;

        entry; then start;
        state start;
        state working {
            entry action prep {
                first start;
                then action a { assign seq := seq * 10 + 1; }
                then action b { assign seq := seq * 10 + 2; }
                then done;
            }
            exit action wrap {
                action c { assign seq := seq * 10 + 3; }
                action d { assign seq := seq * 10 + 4; }
                first c then d;
            }
        }
        succession first start then working;
        succession first working then done;
    }
}
