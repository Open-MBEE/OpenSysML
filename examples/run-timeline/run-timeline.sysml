package RunTimeline {
    private import SI::*;

    item def Ping;
    part def Receiver {
        exhibit state modes {
            entry; then listening;
            state listening;
            state received;
            state timedOut;
            transition first listening accept Ping then received;
            transition first listening accept after 4 [s] then timedOut;
        }
        }

        part def Sender {
            part receiver : Receiver;
            exhibit state modes parallel {
            state transmission {
                entry; then waiting;
                    state waiting;
                    state sending {
                        entry send new Ping() to receiver;
                }
                state sent;
                transition first waiting accept after 1 [s] then sending;
                transition first sending accept after 2 [s] then sent;
            }
            state power {
                entry; then charged;
                state charged;
                state low;
            transition first charged accept after 4 [s] then low;
            transition first low accept after 4 [s] then charged;
            }
        }
        }

        part def Mission {
            part sender : Sender;
        }

    part mission : Mission;
}
