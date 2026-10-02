action def Timing {
    @Simulation::Configuration {
        timeVariable = "simtime";
    }
    part target : Executive;
}
part def Executive {
    attribute t0 : ScalarValues::Real;
    attribute tdiff : ScalarValues::Real;
    attribute tStep : ScalarValues::Real default = 0.0;
    attribute steps : ScalarValues::Integer default = 0;
    state def 'Executive Behavior' {
        in ref context : Executive[1];
        entry action stamp {
            assign context.t0 := localClock.currentTime;
        }
        then Aligning;
        state Aligning;
        state Phasing;
        state Counting;
        state Idle;
        transition first Aligning accept after 3.0 [SI::s]
            do action diffTime {
                attribute t1 : ScalarValues::Real;
                assign t1 := localClock.currentTime;
                attribute tdiffOld : ScalarValues::Real;
                assign tdiffOld := context.tdiff;
                assign context.tdiff := t1 - context.t0;
                assign context.tStep := context.tdiff - tdiffOld;
            }
            then Phasing;
        transition first Phasing accept after 2.0 [SI::s]
            do action reset {
                assign context.tdiff := 0;
                assign context.tStep := context.tdiff + context.steps;
            }
            then Counting;
        transition first Counting
            do action count {
                assign context.steps := context.steps + 1;
                assign context.tdiff := context.tdiff + context.steps;
            }
            then Idle;
    }
}
