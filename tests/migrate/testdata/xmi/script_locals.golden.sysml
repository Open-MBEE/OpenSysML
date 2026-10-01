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
            do action javaDiff {
                /* body not migrated (the name "tdiffOld" resolves to nothing readable: nothing visible from Executive::Executive Behavior::<Region>::<Transition>::javaDiff is called tdiffOld) {Java}:
                 * tdiffOld = tdiff;
                 * tdiff = tdiff + 1.0;
                 */
            }
            then Counting;
        transition first Counting
            do action readFirst {
                /* body not migrated (the name "stepsOld" resolves to nothing readable: nothing visible from Executive::Executive Behavior::<Region>::<Transition>::readFirst is called stepsOld) {JavaScript}:
                 * steps = steps + stepsOld;
                 * stepsOld = steps;
                 */
            }
            then Idle;
    }
}
