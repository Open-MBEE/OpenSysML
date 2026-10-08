package Termination {
    private import ScalarValues::*;
    private import SI::*;

    part def Stopper {
        exhibit state modes {
            entry; then waiting;

            state waiting {
                do action stop {
                    first start;
                    then action wait accept after 2 [s];
                    then terminate this;
                }
            }
        }
    }

    part def Runner {
        exhibit state modes {
            entry; then waiting;

            state waiting;
            state finished;

            transition first waiting accept after 10 [s] then finished;
        }
    }

    part def Mission {
        part stopper : Stopper;
        part runner : Runner;
    }

    part mission : Mission;
}
