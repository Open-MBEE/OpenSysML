// Of two referenced requirements bound to the same subject, one within its cap
// and one over it, the one over it is violated and so is the requiring one.
package test {
    private import ScalarValues::Real;

    part def Wagon {
        attribute tare : Real;
        attribute cargo : Real;
    }

    part wagon1 : Wagon {
        attribute :>> tare = 800.0;
        attribute :>> cargo = 150.0;
    }

    requirement def MassCap {
        attribute actual : Real;
        attribute cap : Real;
        require constraint { actual <= cap }
    }

    requirement def WagonMassCap :> MassCap {
        subject w : Wagon;
        attribute :>> actual = w.tare + w.cargo;
    }

    requirement ladenLimit : WagonMassCap {
        attribute :>> cap = 1000.0;
    }

    requirement emptyLimit : WagonMassCap {
        attribute :>> cap = 900.0;
    }

    requirement spec {
        subject unit = wagon1;
        require ladenLimit { in w = unit; }
        require emptyLimit { in w = unit; }
    }
}
