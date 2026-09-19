package test {
    // An entry/do/exit action may be given by reference to an action declared
    // elsewhere (SysML 7.17.6 reference subsetting, 7.19.3
    // StateSubactionMembership): `entry Bump;` is `entry perform Bump;` with the
    // keyword elided, so each of the three subactions invokes Bump once and
    // `counter` ends at 31 only if all three ran.
    action def Bump {
        inout counter : Integer;

        first start;

        action bumping {
            assign counter := counter + 10;
        }

        done;

        succession first start then bumping;
        succession first bumping then done;
    }

    state TestSubactionReference {
        attribute counter : Integer = 1;

        entry; then init;
        state init;

        state active {
            entry Bump;
            do Bump;
            exit Bump;
        }


        succession first init then active;
        succession first active then done;
    }
}
