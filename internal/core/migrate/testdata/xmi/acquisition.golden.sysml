part def 'Group 0' {
    /* applied stereotype «SimulationConfig»: durationSimulationMode = random; numberOfRuns = 6; timeVariableName = simtime */
}
part def TCS {
    attribute i : ScalarValues::Integer default = 0;
    attribute Retries : ScalarValues::Integer default = 4;
    attribute GS_Found : ScalarValues::Boolean default = false;
    attribute ditSetup : ScalarValues::Real default = 2.5;
}
part def Observatory {
    part tcs : TCS;
    attribute Time_Acq_Total : ScalarValues::Real default = 0.0;
    action def Acquire {
        attribute 'Time_Loop start' : ScalarValues::Real default = 0.0;
        attribute Time_Loop : ScalarValues::Real default = 0.0;
        first start then 'start timer';
        action 'start timer' {
            assign this.Time_Acq_Total := localClock.currentTime;
        }
        first 'start timer' then stamp;
        action stamp {
            assign 'Time_Loop start' := localClock.currentTime;
        }
        first stamp then 'first attempt';
        action 'first attempt' {
            assign this.tcs.i := 1;
            assign this.tcs.GS_Found := false;
        }
        first 'first attempt' then 'merge';
        merge 'merge';
        first 'merge' then wait;
        action wait accept after this.tcs.ditSetup [SI::s];
        first wait then attempt;
        action attempt;
        first attempt then 'decide';
        decide 'decide';
        first 'decide' if this.tcs.i >= this.tcs.Retries then 'guide star found';
        first 'decide' if not this.tcs.GS_Found and this.tcs.i < this.tcs.Retries then 'next attempt';
        action 'guide star found' {
            assign this.tcs.GS_Found := true;
        }
        first 'guide star found' then stamp2;
        action stamp2 {
            assign Time_Loop := localClock.currentTime - 'Time_Loop start';
        }
        first stamp2 then 'stop timer';
        action 'next attempt' {
            assign this.tcs.i := this.tcs.i + 1;
        }
        first 'next attempt' then 'merge';
        action 'stop timer' {
            assign this.Time_Acq_Total := localClock.currentTime - this.Time_Acq_Total;
        }
        first 'stop timer' then final;
        action final terminate;
        /* partition 'Telescope Control' represents Observatory::tcs: first attempt, attempt, decide, guide star found, next attempt */
    }
}
/* not migrated: TimeEvent (_te1) — no trigger refers to the event; the time event's time is not written: the duration "ditSetup s" is neither a number with a time unit nor an expression: names ditSetup, which nothing visible from Model is called */
/* not migrated: TimeEvent (_te2) — no trigger refers to the event; the time event's time is not written: the duration "ditSetup s" is neither a number with a time unit nor an expression: names ditSetup, which nothing visible from Model is called */
