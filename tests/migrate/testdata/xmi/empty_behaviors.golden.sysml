item def Go;
part def Log;
part def Gate {
    attribute count : ScalarValues::Integer default = 0;
    action def Reset;
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
                /* body not migrated (the statement "CALL FLUSH(count)" is not an assignment of a v2 expression (language Fortran)) {Fortran}:
                 * CALL FLUSH(count)
                 */
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
                    /* not migrated: DestroyObjectAction 'drop log' — no v2 form for a UML DestroyObjectAction */
                }
            }
            do action poll {
                /* body not migrated (the statement "CALL POLL(count)" is not an assignment of a v2 expression (language Fortran)) {Fortran}:
                 * CALL POLL(count)
                 */
            }
            exit action close;
        }
        state Cool {
            entry action vent {
                /* body not migrated (the statement "CALL VENT(count)" is not an assignment of a v2 expression (language Fortran)) {Fortran}:
                 * CALL VENT(count)
                 */
            }
            do action wait;
            exit action seal {
                first start then 'new log';
                action 'new log' {
                    /* not migrated: CreateObjectAction 'new log' — no v2 form for a UML CreateObjectAction */
                }
            }
        }
        state Warm;
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
                /* body not migrated (the statement "CALL WARM(count)" is not an assignment of a v2 expression (language Fortran)) {Fortran}:
                 * CALL WARM(count)
                 */
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
                /* body not migrated (the statement "CALL RESTART(count)" is not an assignment of a v2 expression (language Fortran)) {Fortran}:
                 * CALL RESTART(count)
                 */
            }
            then Idle;
        transition first Off accept Go
            do action : Reset
            then Hot;
        transition first Hot
            do action : Reset
            then Idle;
    }
    exhibit state cycle : Cycle;
}
