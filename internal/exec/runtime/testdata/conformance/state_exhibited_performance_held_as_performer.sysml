package test {
    attribute def Wake;
    part def Tug {
        exhibit state warmup {
            entry; then waiting;
            state waiting;
            state warm;
            transition first waiting accept Wake then warm;
        }
        exhibit state sup {
            entry; then armed;
            state armed;
        }
        first warmup then sup;
    }
    part def Scenario {
        part tug : Tug;
    }
    part scenario : Scenario;
}
