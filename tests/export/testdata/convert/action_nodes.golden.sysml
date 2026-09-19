package ActionNodes {
    attribute def Command;
    port def Radio;
    action def Brake;
    action def Drive {
        in attribute speed;
        attribute log;
        port antenna : Radio;
        action def Stop;
        first start;
        action brake : Brake;
        fork split;
        join sync;
        merge gather;
        decide pick;
        perform brake;
        action Brake;
        assign log := speed;
        send new Command() to antenna;
        send new Command() via antenna;
        accept when speed > 0;
        terminate;
        terminate brake;
        done;
        succession first start then brake;
        succession first brake then done;
    }
}
