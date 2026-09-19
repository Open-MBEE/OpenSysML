package test {
    private import ScalarValues::*;

    calc def Twice {
        in k : Real;
        out d = k * 2.0;
    }

    action run {
        attribute i : Integer = 0;
        attribute log : Real = 0.0;
        calc t : Twice { in k = i; }

        first start;
        action bump {
            assign i := i + 1;
            assign log := log + t.d;
        }
        decide check;
        done;

        succession first start then bump;
        succession first bump then check;
        succession first check if t.d < 6.0 then bump;
        succession first check if t.d >= 6.0 then done;
    }
}
