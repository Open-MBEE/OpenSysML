package Loops {
    item def Ping;
    item def Pong;
    port def CommandPort { in item request : Ping; out item reply : Pong; }
    state def Responder {
        port command : CommandPort;
        port response : ~CommandPort;
        state idle;
        state replied;
        transition first idle accept Ping via command do send new Pong() via response then replied;
    }
    part def Tug {
        port command : CommandPort;
        port response : ~CommandPort;
        exhibit state m : Responder;
        bind m.response = response;
        interface wire connect response to command;
    }
}
