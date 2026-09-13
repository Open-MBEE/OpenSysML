package Test {
    private import ScalarValues::*;

    // A state's inline do action body runs forks, joins, decisions with guards
    // and a nested node's own flow as a standalone action body does: the fork's
    // branches each record themselves, the join waits for both, the decision
    // takes the guard that holds, and the nested node runs its own flow to its
    // end before the token moves on.
    state def Machine {
        attribute left : Integer = 0;
        attribute right : Integer = 0;
        attribute joined : Integer = 0;
        attribute taken : Integer = 0;
        attribute seq : Integer = 0;

        entry; then working;
        state working {
            do action ops {
                first start;
                then fork split;
                action a { assign left := 1; }
                action b { assign right := 2; }
                join sync;
                succession first split then a;
                succession first split then b;
                succession first a then sync;
                succession first b then sync;
                succession first sync then tally;
                action tally {
                    first start;
                    then action sum { assign joined := left + right; }
                    then action mark { assign seq := joined * 10; }
                    then done;
                }
                succession first tally then check;
                decide check;
                if joined > 2 then big;
                else small;
                action big { assign taken := 1; }
                action small { assign taken := 2; }
                succession first big then done;
                succession first small then done;
            }
        }
        succession first working then done;
    }
}
