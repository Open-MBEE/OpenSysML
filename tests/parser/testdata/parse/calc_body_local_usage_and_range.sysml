package CalcBodyLocalUsage {
    calc def Rates {
        in px : Real;
        in pv : Real;
        out dx = pv;
        out dv = 0.0 - px;
    }

    calc def Propagate {
        in n : Integer;
        attribute x : Real = 1.0;
        attribute v : Real = 0.0;
        attribute i : Integer = 0;
        while i < n {
            calc r : Rates {
                in px = x;
                in pv = v;
            }
            assign x := x + r.dx;
            assign v := v + r.dv;
            assign i := i + 1;
        }
        return : Real = x;
    }

    calc def Branching {
        in k : Real;
        if k > 0.0 {
            calc up : Rates {
                in px = k;
                in pv = 1.0;
            }
            return : Real = up.dx;
        } else {
            calc down : Rates {
                in px = 0.0 - k;
                in pv = 2.0;
            }
            return : Real = down.dv;
        }
    }

    calc def Squares {
        in n : Integer;
        attribute squares = (1..n)->collect { in i; i * i };
        attribute window = (2..3)->collect { in i; i };
        attribute total : Integer = 0;
        for i in 1..n {
            assign total := total + i;
        }
        return : Integer = total;
    }
}
