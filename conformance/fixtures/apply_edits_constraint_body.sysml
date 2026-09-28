package Demo {
    constraint def ConstraintType;
    action def Work;
    state def Machine {
        state active;
    }
    part def Host {
        state idle : Machine;
    }
}
