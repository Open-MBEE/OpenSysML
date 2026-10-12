package SerialPairing {
    private import ScalarValues::*;
    private import SI::*;

    package Alpha {
        item def Ping {
            attribute seq : Integer;
        }
    }

    package Beta {
        item def Ping {
            attribute seq : Integer;
        }
    }

    port def PingPort {
        out item alpha : Alpha::Ping;
        out item beta : Beta::Ping;
    }

    part def Sender {
        port p : PingPort;
    }

    part def Receiver {
        port p : ~PingPort;

        exhibit state listening {
            entry; then waiting;

            state waiting;
            state received;

            transition first waiting accept order : Beta::Ping via p then received;
        }
    }

    part def Mission {
        part alpha : Sender {
            exhibit state dispatch {
                entry; then sent;

                state sent {
                    entry send new Alpha::Ping(seq = 1) via p;
                }
            }
        }

        part beta : Sender {
            exhibit state dispatch {
                entry; then waiting;

                state waiting;
                state sent {
                    entry send new Beta::Ping(seq = 2) via p;
                }

                transition first waiting accept after 1 [s] then sent;
            }
        }

        part receiver : Receiver;

        connect alpha.p to receiver.p;
        connect beta.p to receiver.p;
    }

    part mission : Mission;
}
