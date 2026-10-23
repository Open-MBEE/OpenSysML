package test {
    private import ScalarValues::*;
    private import SI::*;
    attribute def Ping;
    port def CommandPort { in item cmd : Ping; }
    state def Supervisor {
        port command : CommandPort;
        entry; then armed;
        state armed;
        state pinged;
        state expired;
        transition first armed accept Ping via command then pinged;
        transition first armed accept after 10 [s] then expired;
    }
    part def Tug {
        port command : CommandPort;
        exhibit state sup : Supervisor;
        bind sup.command = command;
    }
    part def Scenario {
        part tug : Tug;
    }
    part scenario : Scenario;
}
