package Payloads {
    attribute def Prio;
    item def Fuel;
    part def A {
        out item i : Fuel;
    }
    part def B {
        in item j : Fuel;
    }
    part def Ctx {
        part x : A;
        part y : B;
        flow of fuel from x.i to y.j {
            attribute w : Prio;
        }
        flow m of cmd from x.i to y.j;
        flow of cmd from x.i to y.j {
            attribute w : Prio;
        }
    }
}
