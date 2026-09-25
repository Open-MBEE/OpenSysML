package OtherPkg {
    item def BudgetLedger;
    enum def Tempo {
        operative;
        planning;
    }
    package Inner {
        part def Wheel {
            attribute size;
        }
        part wheels : Wheel;
    }
    part spare : Inner::Wheel;
}
package R90 {
    public import OtherPkg::*;
    alias Ledger for OtherPkg::BudgetLedger;
    part def A {
        item custodyBudget : OtherPkg::BudgetLedger;
        attribute t : Tempo = Tempo::operative;
        item b2 : BudgetLedger;
        item b3 : Ledger;
        part w : Inner::Wheel :> Inner::wheels;
        ref spare ::> spare;
        attribute size;
        attribute s = w.size;
        part unresolved : Elsewhere::Missing;
    }
    part def B :> A {
        part :>> w : Inner::Wheel;
    }
    part def G1 {
        part x;
    }
    part def G2 {
        part x;
    }
    part def H :> G1, G2 {
        part :>> G2::x;
    }
    state def Run {
        entry; then idle;
        state idle;
        transition idle then Done::done;
    }
    state def Done {
        state done;
    }
}
package Meta {
    metadata def Safety;
    #Safety part def Tagged;
    part def Plain;
}
package Filtered {
    public import Meta::*[@Safety];
    part def C {
        attribute other : Tagged;
        filter @Safety;
    }
}
