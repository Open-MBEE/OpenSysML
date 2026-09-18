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
    alias Ledger for BudgetLedger;
    part def A {
        item custodyBudget : BudgetLedger;
        attribute t : Tempo = Tempo::operative;
        item b2 : BudgetLedger;
        item b3 : BudgetLedger;
        part w : Inner::Wheel subsets Inner::wheels;
        ref spare references spare;
        attribute size;
        attribute s = w.size;
        part unresolved : Elsewhere::Missing;
    }
    part def B specializes A {
        part : Inner::Wheel redefines w;
    }
    part def G1 {
        part x;
    }
    part def G2 {
        part x;
    }
    part def H specializes G1, G2 {
        part redefines G2::x;
    }
    state def Run {
        entry;
        then idle;
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
    public import Meta::* [(@ Safety)];
    part def C {
        attribute other : Tagged;
        filter (@ Safety);
    }
}
