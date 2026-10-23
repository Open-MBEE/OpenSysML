package test {
    private import ScalarValues::*;
    attribute def Ping;
    part def Tug {
        attribute armings : Natural = 0;
        exhibit state sup {
            entry; then armed;
            state armed {
                entry action arm { assign armings := armings + 1; }
            }
            state pinged;
            transition first armed accept Ping then pinged;
        }
    }
    part def Scenario {
        part tug : Tug;
    }
    part scenario : Scenario;
}
