package Payloads {
    attribute def Prio;
    item def Fuel;
    part def A { out item i : Fuel; }
    part def B { in item j : Fuel; }
    part def Ctx {
        part x : A; part y : B;
        flow of fuel : Fuel from x.i to y.j { attribute w : Prio; }
        message m of cmd : Prio from x.i to y.j;
        message of cmd : Prio = m.cmd from x.i to y.j { attribute w : Prio; }
    }
}
