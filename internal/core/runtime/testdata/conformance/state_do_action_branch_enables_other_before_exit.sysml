package test {
    private import ScalarValues::*;
    private import SI::*;

    state def Machine {
        attribute flag : Integer = 0;
        attribute seen : Integer = 0;

        entry; then running;
        state running {
            do action poll {
                first start;
                then fork split;
                succession first split then setter;
                succession first split then watcher;

                action setter {
                    first start;
                    then action wait accept after 1 [s];
                    then action raise assign flag := flag + 1;
                }

                action watcher {
                    first start;
                    then action watch accept when flag > seen;
                    then action note assign seen := seen + 1;
                }
            }
        }
        transition first running accept when flag > 0 then finished;
        state finished;
    }
}
