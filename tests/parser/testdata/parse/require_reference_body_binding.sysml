package Freight {
    private import ScalarValues::*;
    part def Wagon {
        attribute tare : Real;
        attribute cargo : Real;
    }
    requirement def WagonMassCap {
        subject w : Wagon;
        attribute actual : Real = w.tare + w.cargo;
        attribute cap : Real;
        assume constraint { w.cargo > 0 }
        require constraint { actual <= cap }
    }
    requirement ladenLimit : WagonMassCap {
        attribute :>> cap = 1000;
    }
    requirement wagonSpec {
        subject unit : Wagon;
        require ladenLimit { in w = unit; }
        assume ladenLimit { in w = unit; }
    }
}
