package Demo {
    metadata def FailureMode { attribute severity : ScalarValues::Integer; }
    part def Pump;
    metadata pump_fm : FailureMode about Pump { severity = 2; }
}
