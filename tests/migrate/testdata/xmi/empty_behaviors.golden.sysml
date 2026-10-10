item def Go;
item def Stop;
occurrence def Log;
occurrence def Gate {
    attribute count : ScalarValues::Integer default = 0;
    action def Reset;
    state def Blink {
        entry; then On;
        state On;
    }
    state def Cycle {
        entry; then Idle;
        state Idle {
            entry action prime;
            do action spin {
                first start then 'new log';
                action 'new log' {
                    /* not migrated: CreateObjectAction 'new log' — no v2 form for a UML CreateObjectAction */
                }
            }
            exit action flush {
                rep language "Fortran" /* CALL FLUSH(count)*/
            }
        }
        state Busy {
            entry action open {
                first start then 'fork';
                fork 'fork';
                first 'fork' then 'new log';
                first 'fork' then 'drop log';
                action 'new log' {
                    /* not migrated: CreateObjectAction 'new log' — no v2 form for a UML CreateObjectAction */
                }
                action 'drop log' {
                    /* not migrated: DestroyObjectAction 'drop log' — the action has no target pin */
                }
                metadata MigrationMetadata::SynthesizedName about 'fork';
                metadata MigrationMetadata::StandIn about 'fork';
            }
            do action poll {
                rep language "Fortran" /* CALL POLL(count)*/
            }
            exit action close;
        }
        state Cool {
            entry action vent {
                rep language "Fortran" /* CALL VENT(count)*/
            }
            do action wait;
            exit action seal {
                first start then 'new log';
                action 'new log' {
                    /* not migrated: CreateObjectAction 'new log' — no v2 form for a UML CreateObjectAction */
                }
            }
        }
        state Warm {
            /* entry action Gate::Blink is written as a state def, which no state runs */
        }
        state Cold;
        state Off;
        state Hot;
        transition first Idle accept Go
            do action effect { }
            then Busy;
        transition first Busy accept Go
            do action effect {
                first start then 'new log';
                action 'new log' {
                    /* not migrated: CreateObjectAction 'new log' — no v2 form for a UML CreateObjectAction */
                }
            }
            then Cool;
        transition first Cool accept Go
            do action effect {
                rep language "Fortran" /* CALL WARM(count)*/
            }
            then Warm;
        transition first Warm
            do action effect { }
            then Cold;
        transition first Cold
            do action effect {
                first start then 'new log';
                action 'new log' {
                    /* not migrated: CreateObjectAction 'new log' — no v2 form for a UML CreateObjectAction */
                }
            }
            then Off;
        transition first Off
            do action effect {
                rep language "Fortran" /* CALL RESTART(count)*/
            }
            then Idle;
        transition first Off accept Go
            do action : Reset
            then Hot;
        transition first Hot
            do action : Reset
            then Idle;
        /* do action Gate::Blink is written as a state def, which no state runs */
        transition first Idle accept Stop
            then Off;
    }
    exhibit state cycle : Cycle;
}
