action def Timing {
    @Simulation::Configuration {
        timeVariable = "simtime";
    }
    part target : Executive;
}
part def Executive {
    attribute t0 : ScalarValues::Real default = 0.0;
    attribute tdiff : ScalarValues::Real default = 0.0;
    attribute tStep : ScalarValues::Real default = 0.0;
    attribute steps : ScalarValues::Integer default = 0;
    state def 'Executive Behavior' {
        in ref context : Executive[1];
        entry action stamp {
            rep language "JavaScript" /* t0 = simtime;*/
            assign context.t0 := localClock.currentTime;
        }
        then Aligning;
        state Aligning;
        state Phasing;
        state Counting;
        state Idle;
        transition first Aligning accept after 3.0 [SI::s]
            do action diffTime {
                rep language "JavaScript" /* t1 = simtime;
                 * tdiffOld = tdiff;
                 * tdiff = (t1 - t0);
                 * tStep = tdiff - tdiffOld;
                 * print("step " + tStep);*/
                attribute t1 : ScalarValues::Real;
                assign t1 := localClock.currentTime;
                attribute tdiffOld : ScalarValues::Real;
                assign tdiffOld := context.tdiff;
                assign context.tdiff := t1 - context.t0;
                assign context.tStep := context.tdiff - tdiffOld;
            }
            then Phasing;
        transition first Phasing accept after 2.0 [SI::s]
            do action javaDiff {
                rep language "Java" /* tdiffOld = tdiff;
                 * tdiff = tdiff + 1.0;*/
            }
            then Counting;
        transition first Counting
            do action readFirst {
                rep language "JavaScript" /* steps = steps + stepsOld;
                 * stepsOld = steps;*/
            }
            then Idle;
    }
}
