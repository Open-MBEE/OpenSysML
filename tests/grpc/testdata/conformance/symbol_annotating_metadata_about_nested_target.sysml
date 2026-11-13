package AboutMetadata {
    metadata def FailureMode {
        attribute severity : ScalarValues::Integer;
    }
    #FailureMode part def PumpP;
    part def PumpB { @FailureMode { severity = 3; } }
    part def PumpA;
    metadata pumpA_fm : FailureMode about PumpA { severity = 2; }
    part def Pump {
        part motorA;
        state def PumpStates {
            state overheatedA;
        }
        action pumpFluid {
            in pressureA : ScalarValues::Real;
        }
    }
    metadata motorA_fm : FailureMode about Pump::motorA { severity = 4; }
    metadata shared_fm : FailureMode about Pump::PumpStates::overheatedA, Pump::pumpFluid::pressureA, PumpA;
}
