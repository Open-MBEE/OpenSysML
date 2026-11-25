package MetadataAnnotations {
    metadata def FailureMode {
        attribute severity : ScalarValues::Integer;
    }
    part def Pump {
        state def PumpStates {
            entry; then idle;
            state idle;
            state jammed { @FailureMode { severity = 3; } }
            #FailureMode state overheated;
            state stalled { metadata fm : FailureMode; }
        }
        action pumpFluid {
            in flowRate : ScalarValues::Real { @FailureMode { severity = 2; } }
            in #FailureMode pressure : ScalarValues::Real;
        }
    }
}
