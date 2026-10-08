// A transition nested in the state keeping a signal takes it: `outer` keeps
// Continue through the standard deferred-signal encoding, but its substate
// `inner` has a transition on Continue, which fires and consumes the
// occurrence, so nothing is kept. Finish then leaves `outer`, whose exit action
// has nothing to send back.
package Test {
    item def Continue;
    item def Finish;

    state NestedTransitionWins {
        entry; then outer;
        state outer {
            item deferred : Continue[*] ordered;
            do action buffer {
                first start then receive;
                action receive accept kept : Continue;
                then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
                then receive;
            }
            exit action flush {
                for kept in deferred { send kept to self; }
                then action clear { assign deferred := (); }
            }
            entry; then inner;
            state inner;
            state seen;
            transition first inner accept Continue then seen;
        }

        transition first outer accept Finish then done;
    }
}
