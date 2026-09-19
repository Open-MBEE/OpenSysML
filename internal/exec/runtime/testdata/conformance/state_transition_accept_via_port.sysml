package StateAcceptViaPort {
    private import ScalarValues::*;

    item def Ping;

    port def PingPort {
        in item ping : Ping;
    }

    state Radio {
        attribute received : Integer = 0;

        port outPort : PingPort;
        port inPort : PingPort;
        connect outPort to inPort;

        entry; then start;
        state start;

        state waitingGSPing {
            entry send new Ping() via outPort;
        }
        state transmitting;

        succession first start then waitingGSPing;

        // The trigger is written on its own line, naming the port the accepted
        // occurrence must arrive at, and the transition carries a name of its own.
        transition ping first waitingGSPing
            accept Ping via inPort
            do assign received := 1
            then transmitting;

        transition first transmitting then done;
    }
}
