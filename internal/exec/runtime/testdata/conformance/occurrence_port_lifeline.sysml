package Sequences {
    item def Ping;
    item def Pong;
    port def Rx;

    occurrence def PortExchange {
        part sender {
            event occurrence ping_sent;
            then event occurrence pong_received;
        }
        port rx : Rx {
            event occurrence ping_received;
            then event occurrence pong_sent;
        }
        message ping of Ping[1] from sender.ping_sent to rx.ping_received;
        message pong of Pong[1] from rx.pong_sent to sender.pong_received;
    }
}
