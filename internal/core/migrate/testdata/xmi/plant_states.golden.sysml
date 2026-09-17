item def Go;
item def Stop;
item def Hold;
item def Resume;
item def Next;
item def Enter;
item def Finish;
item def Split;
item def Route;
item def Ping;
item def Bump;
item def Unused;
part def Plant {
    attribute count : ScalarValues::Integer default = 0;
    state def Line {
        attribute dawn : Time::TimeInstantValue = 21600.0 [SI::s];
        entry; then Idle;
        state Idle;
        state Work {
            entry; then Prep;
            exit action tally {
                assign this.count := this.count + 1;
            }
            state Prep;
            state Run;
            history last;
            deep history deepest;
            transition first Prep accept Next then Run;
            transition first last then Prep;
        }
        state Pause;
        state Cell : CellMachine;
        junction route;
        fork spread;
        join gather;
        state Both {
            /* no default entry: the regions 'a', 'b' have no initial pseudostate, so only a fork or a transition naming a nested state enters the regions */
            state regions parallel {
                state a {
                    /* the region has no initial pseudostate: nothing enters it */
                    state A1;
                    state A2;
                    transition first A1 accept Next then A2;
                }
                state b {
                    /* the region has no initial pseudostate: nothing enters it */
                    state B1;
                    state B2;
                    transition first B1 accept Next then B2;
                }
            }
            transition first regions then done;
        }
        transition first Idle accept Go then Work;
        transition first Work::Run accept Stop then Idle;
        transition first Idle accept Resume then Work::Run;
        transition first Work accept Hold then Pause;
        transition first Pause accept Resume then Work::last;
        transition first Idle accept Enter then Cell::warmStart;
        transition first Cell::spent then Idle;
        transition first Idle accept Route then route;
        transition first route if this.count < 2 then Work;
        transition first route then done;
        transition first Idle accept Split then spread;
        transition first spread then Both::regions::a::A1;
        transition first spread then Both::regions::b::B1;
        transition first Both::regions::a::A2 then gather;
        transition first Both::regions::b::B2 then gather;
        transition first gather then Idle;
        transition first Idle accept Ping then Idle;
        transition first Work accept Bump then Work::Prep;
        transition first Pause accept at dawn then Idle;
        /* transition (_tNever) from Pause to done not migrated — a trigger is dropped: the time event's time is not written: the instant "next Tuesday" is not a number with a time unit */
    }
    state def CellMachine {
        state warmStart;
        state spent;
        entry; then Cold;
        state Cold;
        state Hot;
        transition first Cold accept Next then Hot;
        transition first warmStart then Hot;
        transition first Hot accept Finish then spent;
    }
    exhibit state line : Line;
}
