package test {
    item def Ping;
    port def PingPort { in item ping : Ping; }
    part def Car { port p : PingPort; }
    part def Fleet {
        part cars : Car[0..*];
        port out : ~PingPort;
        connect out to cars.p;
        perform action build {
            first start;
            then action ping {
                send new Ping() to cars#(2);
                send new Ping() via out to cars#(2);
            }
            then done;
        }
    }
}
