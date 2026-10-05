// `assume r { in w = unit; }` assumes the referenced requirement rather than
// requiring it: its violated condition is trusted, and only the required
// reference decides.
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
        assume emptyLimit { in w = unit; }
        require ladenLimit { in w = unit; }
    }
}
