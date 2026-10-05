// A `require r { in w = unit; }` member binds the referenced requirement's
// subject to the satisfaction subject, so an attribute the referenced
// requirement redefines in terms of its subject (`actual = w.tare + w.cargo`)
// is computed from the wagon the `satisfy` names: one wagon is within both
// caps, the other exceeds the empty-running cap alone.
package Freight {
    private import ISQ::*;
    private import SI::*;

    part def Wagon {
        attribute tare : MassValue;
        attribute cargo : MassValue;
    }

    part wagon1 : Wagon {
        attribute :>> tare = 800 [kg];
        attribute :>> cargo = 150 [kg];
        satisfy wagonSpec by wagon1;
    }

    part wagon2 : Wagon {
        attribute :>> tare = 800 [kg];
        attribute :>> cargo = 50 [kg];
        satisfy wagonSpec by wagon2;
    }

    requirement wagonSpec {
        subject unit : Wagon;
        require ladenLimit { in w = unit; }
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

    requirement ladenLimit : WagonMassCap {
        attribute :>> cap = 1000 [kg];
    }

    requirement emptyLimit : WagonMassCap {
        attribute :>> cap = 900 [kg];
    }
}
