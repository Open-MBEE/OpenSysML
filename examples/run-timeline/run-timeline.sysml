package RunTimeline {
    private import ScalarValues::*;
    private import SI::*;

    item def Ping {
        attribute seq : Integer;
    }

    item def Ack {
        attribute seq : Integer;
    }

    port def ControllerLink {
        out item ping : Ping;
        in item ack : Ack;
    }

    port def InstrumentLink {
        in item ping : Ping;
        out item ack : Ack;
    }

    part def Controller {
        port link : ControllerLink;

        exhibit state modes parallel {
            state communication {
                entry; then waiting;
                state waiting;
                state sending {
                    entry send new Ping(seq = 1) via link;
                }
                state acknowledged;

                transition first waiting accept after 1 [s] then sending;
                transition first sending accept Ack via link then acknowledged;
                transition first acknowledged accept after 1 [s] then waiting;
            }

            state power {
                entry; then charged;
                state charged;
            }
        }
    }

    part def Instrument {
        port link : InstrumentLink;

        exhibit state modes {
            entry; then waiting;
            state waiting;
            state acknowledged;

            transition first waiting
                accept ping : Ping via link
                do send new Ack(seq = ping.seq) via link
                then acknowledged;
            transition first acknowledged accept after 1.5 [s] then waiting;
        }
    }

    part def Mission {
        part controller : Controller;
        part instrument : Instrument;

        connect controller.link to instrument.link;
    }

    part mission : Mission;
}
