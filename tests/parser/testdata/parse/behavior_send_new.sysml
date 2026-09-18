action def Transmitter {
    port antenna : Antenna;
    part ground : Station;
    action begin;
    first begin;
    then send new Telemetry(frames = 3.0) via antenna;
    then send new Command(7) to ground;
    then send new Ping() via antenna to ground;
}
