package test {
    private import ScalarValues::*;
    action def Decide {
        in limit : Real;
        out go : Boolean = limit <= 10.0;
    }
    action def Sequence {
        in sysLimit : Real;
        action decision : Decide { in limit = sysLimit; }
        then done;
    }
    part site {
        perform action low : Sequence { in sysLimit = 6.8; }
        perform action high : Sequence { in sysLimit = 12.5; }
        attribute lowGo : Boolean = low.decision.go;
        attribute highGo : Boolean = high.decision.go;
    }
}
