package SendNew {
    item def Telemetry {
        attribute frames;
    }
    item def Command;
    port def Radio;
    part def Station;
    action def Downlink {
        port antenna : Radio;
        part ground : Station;
        send new Telemetry(frames = 3) via antenna;
        send new Command(7) to ground;
    }
}
