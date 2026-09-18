part def 'Group 0' {
    /* applied stereotype «SimulationConfig»: durationSimulationMode = random; numberOfRuns = 6; timeVariableName = simtime */
}
part def 'Group 1' {
    /* applied stereotype «SimulationConfig»: durationSimulationMode = max; numberOfRuns = 1 */
}
part def TCS {
    attribute i : ScalarValues::Integer default = 0;
    attribute Retries : ScalarValues::Integer default = 4;
    attribute GS_Found : ScalarValues::Boolean default = false;
    attribute 'Guide Star Lost' : ScalarValues::Boolean default = false;
    attribute ditSetup : ScalarValues::Real default = 2.5;
}
part def Observatory {
    part tcs : TCS;
    attribute Time_Acq_Total : ScalarValues::Real default = 0.0;
    action def Acquire {
        attribute 'Time_Loop start' : ScalarValues::Real [0..1];
        attribute Time_Loop : ScalarValues::Real [0..1];
        attribute 'Time_Between start' : ScalarValues::Real [0..1];
        attribute Time_Between : ScalarValues::Real [0..1];
        attribute 'Time_Attempt start' : ScalarValues::Real [0..1];
        attribute Time_Attempt : ScalarValues::Real [0..1];
        attribute 'Time_Never start' : ScalarValues::Real [0..1];
        attribute Time_Never : ScalarValues::Real [0..1];
        attribute 'Time_Unfinished start' : ScalarValues::Real [0..1];
        attribute Time_Unfinished : ScalarValues::Real [0..1];
        attribute 'Time_Run start' : ScalarValues::Real [0..1];
        attribute Time_Run : ScalarValues::Real [0..1];
        first start then stamp9;
        action stamp9 {
            assign 'Time_Run start' := localClock.currentTime;
        }
        first stamp9 then 'start timer';
        action 'start timer' {
            assign this.Time_Acq_Total := localClock.currentTime;
        }
        first 'start timer' then stamp;
        action stamp {
            assign 'Time_Loop start' := localClock.currentTime;
            assign 'Time_Unfinished start' := localClock.currentTime;
        }
        first stamp then 'first attempt';
        action 'first attempt' {
            assign this.tcs.i := 1;
            assign this.tcs.GS_Found := false;
        }
        first 'first attempt' then stamp3;
        action stamp3 {
            assign 'Time_Between start' := localClock.currentTime;
        }
        first stamp3 then 'merge';
        merge 'merge';
        first 'merge' then stamp5;
        action wait accept after this.tcs.ditSetup [SI::s];
        first wait then attempt;
        action stamp5 {
            assign 'Time_Attempt start' := localClock.currentTime;
        }
        first stamp5 then wait;
        action attempt;
        first attempt then stamp6;
        action stamp6 {
            if 'Time_Attempt start'->SequenceFunctions::notEmpty() {
                assign Time_Attempt := localClock.currentTime - 'Time_Attempt start';
            }
        }
        first stamp6 then 'decide';
        decide 'decide';
        first 'decide' if this.tcs.i >= this.tcs.Retries then stamp4;
        first 'decide' if not this.tcs.GS_Found and not this.tcs.'Guide Star Lost' and this.tcs.i < this.tcs.Retries then 'next attempt';
        first 'decide' if false then stamp7;
        action stamp4 {
            if 'Time_Between start'->SequenceFunctions::notEmpty() {
                assign Time_Between := localClock.currentTime - 'Time_Between start';
            }
        }
        first stamp4 then 'guide star found';
        action 'guide star found' {
            assign this.tcs.GS_Found := true;
        }
        first 'guide star found' then stamp2;
        action stamp2 {
            if 'Time_Loop start'->SequenceFunctions::notEmpty() {
                assign Time_Loop := localClock.currentTime - 'Time_Loop start';
            }
            if 'Time_Never start'->SequenceFunctions::notEmpty() {
                assign Time_Never := localClock.currentTime - 'Time_Never start';
            }
        }
        first stamp2 then merge2;
        action 'next attempt' {
            assign this.tcs.i := this.tcs.i + 1;
        }
        first 'next attempt' then 'merge';
        action stamp7 {
            assign 'Time_Never start' := localClock.currentTime;
        }
        first stamp7 then abort;
        action abort {
            assign this.tcs.GS_Found := false;
        }
        first abort then stamp8;
        action stamp8 {
            if 'Time_Unfinished start'->SequenceFunctions::notEmpty() {
                assign Time_Unfinished := localClock.currentTime - 'Time_Unfinished start';
            }
        }
        first stamp8 then merge2;
        action 'stop timer' {
            assign this.Time_Acq_Total := localClock.currentTime - this.Time_Acq_Total;
        }
        first 'stop timer' then stamp10;
        merge merge2;
        first merge2 then 'stop timer';
        action stamp10 {
            if 'Time_Run start'->SequenceFunctions::notEmpty() {
                assign Time_Run := localClock.currentTime - 'Time_Run start';
            }
        }
        first stamp10 then final;
        action final terminate;
        /* partition 'Telescope Control' represents Observatory::tcs: first attempt, attempt, decide, guide star found, next attempt, abort */
    }
}
/* not migrated: TimeEvent (_te1) — no trigger refers to the event; the time event's time is not written: the duration "ditSetup s" is neither a number with a time unit nor an expression: names ditSetup, which nothing visible from Model is called */
/* not migrated: TimeEvent (_te2) — no trigger refers to the event; the time event's time is not written: the duration "ditSetup s" is neither a number with a time unit nor an expression: names ditSetup, which nothing visible from Model is called */
