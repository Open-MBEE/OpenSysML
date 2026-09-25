item def Go;
item def Enter;
item def Dive;
item def Resume;
item def Skip;
item def Next;
item def Finish;
item def Back;
item def Stop;
item def Pair;
item def Split;
item def Bump;
part def Station {
    attribute trace : ScalarValues::Integer default = 0;
    state def Cycle {
        entry; then Idle;
        state Idle;
        state Work {
            entry action {
                assign this.trace := this.trace * 100 + 11;
            }
            then Prep;
            exit action {
                assign this.trace := this.trace * 100 + 12;
            }
            junction Start;
            /* not migrated: Pseudostate 'Through' — (_tThrough) leads from the entry point straight to the exit point 'Leave' of the same state, crossing it without settling in it; the runtime would then run neither its entry nor its exit behavior */
            junction Leave;
            state Prep {
                entry action {
                    assign this.trace := this.trace * 100 + 13;
                }
            }
            state Run {
                entry action {
                    assign this.trace := this.trace * 100 + 14;
                }
                then Slow;
                exit action {
                    assign this.trace := this.trace * 100 + 15;
                }
                junction Deep;
                junction Out;
                state Slow;
                state Fast {
                    entry action {
                        assign this.trace := this.trace * 100 + 17;
                    }
                }
                transition first Work::Run::Deep
                    do action {
                        assign this.trace := this.trace * 100 + 21;
                    }
                    then Fast;
                transition first Fast accept Back
                    do action {
                        assign this.trace := this.trace * 100 + 22;
                    }
                    then Work::Run::Out;
            }
            history H;
            transition first Prep accept Next then Run;
            transition first Work::Start
                do action {
                    assign this.trace := this.trace * 100 + 23;
                }
                then Run;
            /* not migrated: Transition (_tThrough) — the source 'Through' has no v2 form: (_tThrough) leads from the entry point straight to the exit point 'Leave' of the same state, crossing it without settling in it; the runtime would then run neither its entry nor its exit behavior */
            transition first Run accept Finish
                do action {
                    assign this.trace := this.trace * 100 + 24;
                }
                then Work::Leave;
            transition first Work::Run::Out
                do action {
                    assign this.trace := this.trace * 100 + 25;
                }
                then Prep;
        }
        state Sync {
            entry action {
                assign this.trace := this.trace * 100 + 31;
            }
            then regions;
            exit action {
                assign this.trace := this.trace * 100 + 32;
            }
            fork Both;
            join Gather;
            state regions parallel {
                state a {
                    entry; then A0;
                    state A0;
                    state A1 {
                        entry action {
                            assign this.trace := this.trace * 100 + 33;
                        }
                        exit action {
                            assign this.trace := this.trace * 100 + 34;
                        }
                    }
                    transition first Sync::Both then A1;
                    transition first A0 accept Bump then A1;
                    transition first A1
                        do action {
                            assign this.trace := this.trace * 100 + 37;
                        }
                        then Sync::Gather;
                }
                state b {
                    entry; then B0;
                    state B0;
                    state B1 {
                        entry action {
                            assign this.trace := this.trace * 100 + 35;
                        }
                        exit action {
                            assign this.trace := this.trace * 100 + 36;
                        }
                    }
                    transition first Sync::Both then B1;
                    transition first B0 accept Bump then B1;
                    transition first B1
                        do action {
                            assign this.trace := this.trace * 100 + 38;
                        }
                        then Sync::Gather;
                }
            }
            transition first regions then done;
            metadata MigrationMetadata::SynthesizedName about regions;
        }
        transition first Idle accept Go
            do action {
                assign this.trace := this.trace * 100 + 41;
            }
            then Work::Start;
        transition first Idle accept Go if false then Work;
        transition first Idle accept Enter then Work;
        transition first Idle accept Dive
            do action {
                assign this.trace := this.trace * 100 + 42;
            }
            then Work::Run::Deep;
        transition first Idle accept Resume then Work::H;
        /* not migrated: Transition (_tSkip) — the target 'Through' has no v2 form: (_tThrough) leads from the entry point straight to the exit point 'Leave' of the same state, crossing it without settling in it; the runtime would then run neither its entry nor its exit behavior */
        transition first Work::Leave
            do action {
                assign this.trace := this.trace * 100 + 43;
            }
            then Idle;
        transition first Work accept Stop
            do action {
                assign this.trace := this.trace * 100 + 44;
            }
            then Idle;
        transition first Idle accept Pair then Sync;
        transition first Idle accept Split then Sync::Both;
        transition first Sync::Gather
            do action {
                assign this.trace := this.trace * 100 + 46;
            }
            then Idle;
    }
    exhibit state cycle : Cycle;
}
