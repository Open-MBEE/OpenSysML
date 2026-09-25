package test {
    // A statement of an anonymous inline entry/do/exit body may perform an action
    // (SysML 7.17.6), so a body mixes its own statements with performed actions:
    // counter ends at 121 only if every statement of every body ran in order.
    action def Bump {
        inout counter : Integer;

        first start;

        action bumping {
            assign counter := counter * 10;
        }

        done;

        succession first start then bumping;
        succession first bumping then done;
    }

    state TestAnonymousBodyPerform {
        attribute counter : Integer = 0;

        entry; then init;
        state init;

        state active {
            entry action {
                assign counter := counter + 1;
                perform Bump;
            }
            exit action {
                perform Bump;
                assign counter := counter + 1;
            }
        }


        succession first init then active;
        succession first active then done;
    }
}
