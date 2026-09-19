package test {
    private import ScalarValues::*;

    // `Greet()` passes no arguments, so the callee's default binds `times`,
    // while the bare typed usage reads the enclosing `times`.
    action def Greet {
        in times : Integer = 1;
        out result : Integer;
        first start;
        then action go { assign result := times * 10; }
        then done;
    }

    action outer {
        attribute times : Integer = 5;
        out fromDefault : Integer;
        out fromCaller : Integer;
        first start;
        then action explicit = Greet();
        then action bare : Greet;
        then action fin {
            assign fromDefault := explicit;
            assign fromCaller := bare.result;
        }
        then done;
    }
}
