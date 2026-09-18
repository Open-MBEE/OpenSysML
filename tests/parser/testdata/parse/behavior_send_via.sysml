action def Transmitter {
    port commPort : CommPort;
    attribute data : Data;
    action begin;
    first begin;
    then send Data(data) via commPort;
    then send fullyCharged() to self;
}
