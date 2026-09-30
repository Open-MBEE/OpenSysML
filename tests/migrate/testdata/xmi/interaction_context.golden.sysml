item def Ping;
part def Ctrl {
    attribute got : ScalarValues::Real default = 0.0;
}
part def Rig {
    part ctrl : Ctrl;
    part other : Ctrl;
    action def Pair {
        in ref context : Rig[1];
        action ping send new Ping() to context.other;
        first start then ping;
        first ping then done;
    }
}
part def Plant {
    part rig : Rig;
    state def Duty {
        in ref context : Plant[1];
        entry; then on;
        state on {
            do action : Rig::Pair { in ref :>> context = Duty::context.rig; }
        }
        transition first on then done;
    }
}
