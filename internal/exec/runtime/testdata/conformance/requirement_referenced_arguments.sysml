// A requirement whose subject is bound passes it on to a referenced requirement
// through the reference's body (`in w = unit;`), so the attribute that
// requirement redefines from its subject is read from the bound object.
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

    requirement spec {
        subject unit = wagon1;
        require ladenLimit { in w = unit; }
    }
}
