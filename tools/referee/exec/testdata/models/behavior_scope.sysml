package Behave {
    private import ScalarValues::*;

    part def Counter {
        attribute count : Integer = 0;
    }

    action def Bump {
        in c : Counter;
        out n : Integer = c.count + 1;
    }

    action def Flow {
        first start;
        then action a : Bump { in c = cc; }
        then action b : Bump { in c = cc; }
        then done;
    }

    part cc : Counter;

    state def Modes {
        entry; then state off;
        transition off_to_on
            first off
            then on;
        state on;
    }

    part machine : Counter {
        exhibit state s : Modes;
        perform action p : Bump { in c = cc; }
    }
}
