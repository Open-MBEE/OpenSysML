package Test {
    state Machine {
        attribute log : String = "";

        entry; then S;
        state S parallel {
            exit { assign log := log + "S(exit);"; }
            state R {
                do action work {
                    first start;
                    then action firstStep { assign log := log + "do1;"; }
                    then action secondStep { assign log := log + "do2;"; }
                    then done;
                }
                state A {
                    entry { assign log := log + "A(entry);"; }
                }
            }
        }
        state finished {
            entry { assign log := log + "finished(entry);"; }
        }
        transition first S then finished;
    }
}
