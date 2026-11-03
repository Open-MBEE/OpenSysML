// An assumption of a referenced requirement stays an assumption when the
// reference is required: an empty wagon failing `w.cargo > 0 [kg]` makes
// the satisfaction hold (`allTrue(assumptions()) implies allTrue(constraints())`)
// rather than fail it, whatever `actual <= cap` reads.
package Freight {
    private import ISQ::*;
    private import SI::*;

    part def Wagon {
        attribute tare : MassValue;
        attribute cargo : MassValue;
    }

    part emptyWagon : Wagon {
        attribute :>> tare = 800 [kg];
        attribute :>> cargo = 0 [kg];
        satisfy wagonSpec by emptyWagon;
    }

    requirement wagonSpec {
        subject unit : Wagon;
        require emptyLimit { in w = unit; }
    }

    requirement def MassCap {
        attribute actual : MassValue;
        attribute cap : MassValue;
        require constraint { actual <= cap }
    }

    requirement def WagonMassCap :> MassCap {
        subject w : Wagon;
        attribute :>> actual = w.tare + w.cargo;
        assume constraint { w.cargo > 0 [kg] }
    }

    requirement emptyLimit : WagonMassCap {
        attribute :>> cap = 900 [kg];
    }
}
