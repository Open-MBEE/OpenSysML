package StateMachine {
    attribute def Signal;
    action def Warm;
    port def Bus;
    state def VehicleStates {
        entry;
        state off {
            entry perform Warm;
        }
        state starting {
            entry action ready : Warm;
            do action running : Warm;
            exit perform Warm;
        }
        state active {
            do action monitor : Warm;
            accept sig : Signal then off;
        }
        transition off_to_start first off accept sig : Signal then starting;
        transition first starting if ready then active;
        transition active_to_off first active accept sig : Signal via lane if monitor do action stop : Warm then off;
        port lane : Bus;
    }
    state def Nested {
        state idle {
            state substate {
                entry perform Warm;
            }
        }
    }
    state def Composite {
        entry;
        then launch;
        state launch;
        state waiting;
        entry action {
            perform Warm;
        }
        defer sig, other;
        choice pick;
        junction meet;
        fork split;
        join sync;
        shallow history recent;
        deep history deepest;
        state primary parallel {
            state ready;
        }
    }
}
