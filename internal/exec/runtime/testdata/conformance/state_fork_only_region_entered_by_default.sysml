package ForkOnlyRegion {
    item def Default;
    item def Back;
    item def Fork;
    item def All;
    item def FinishA;
    item def FinishB;
    item def FinishC;

    state Machine {
        attribute log : String = "";
        entry; then init;
        state init;
        state working parallel {
            state left {
                state a { entry { assign log := log + "a(entry) "; } }
                transition first a accept FinishA then done;
            }
            state right {
                state b { entry { assign log := log + "b(entry) "; } }
                transition first b accept FinishB then done;
            }
            state third {
                state c { entry { assign log := log + "c(entry) "; } }
                transition first c accept FinishC then done;
            }
        }
        state finished { entry { assign log := log + "finished(entry) "; } }
        fork split;
        fork all;

        transition first init accept Default then working;
        transition first init accept Fork then split;
        transition first init accept All then all;
        transition first working accept Back then init;
        transition first working then finished;
        transition first split then a;
        transition first split then b;
        transition first all then a;
        transition first all then b;
        transition first all then c;
    }
}
