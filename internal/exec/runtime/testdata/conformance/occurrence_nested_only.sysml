package Sequences {
    item def Ping;
    part def Counter {
        attribute count : ScalarValues::Integer = 0;
        action bump {
            assign count := count + 1;
        }
    }

    occurrence def Inner {
        ref part a : Counter {
            event occurrence sent;
        }
        ref part b : Counter {
            event occurrence got {
                perform action bumps[2] ::> Counter::bump;
            }
            then event occurrence checked {
                assert constraint { count == 2 }
            }
        }
        message ping of Ping[1] from a.sent to b.got;
    }

    occurrence def Outer {
        occurrence run : Inner;
    }
}
