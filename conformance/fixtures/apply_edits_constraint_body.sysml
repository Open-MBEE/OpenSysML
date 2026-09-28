package Demo {
    constraint def ConstraintType;
    action def Work;
    state def Machine {
        state active;
    }
    part def Appliance {
        state idle : Machine;
    }
    part def Host {
        part appliance : Appliance;
    }
}
