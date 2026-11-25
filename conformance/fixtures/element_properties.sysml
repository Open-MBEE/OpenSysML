package Demo {
    port def CmdPort {
        out attribute cmd : ScalarValues::Integer;
        in attribute ack : ScalarValues::Boolean;
    }
    part def Unit { port p : CmdPort; }
    part def Pair :> Unit {
        part u1 : Unit;
        part u2 : Unit;
        connect u1 to u2;
    }
}
