package NestedForkInactiveRegion {
    state Machine {
        attribute log : String = "";
        entry; then init;
        state init;
        state outer parallel {
            state o1 {
                entry; then hold;
                state hold;
                state inner parallel {
                    state left { state a; }
                    state right { state b; }
                }
                fork split;
                transition first split then inner.left.a;
                transition first split then inner.right.b;
            }
            state o2 { state c { entry { assign log := log + "c(entry) "; } } }
        }

        transition first init then outer.o1.split;
    }
}
