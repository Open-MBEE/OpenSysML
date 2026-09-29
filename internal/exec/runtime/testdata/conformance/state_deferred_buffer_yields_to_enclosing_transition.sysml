// A transition out of a state enclosing the one keeping a signal wins over the
// buffer: `outer` has a transition on Ping, so while its substate `waiting`
// would keep Ping through the standard deferred-signal encoding, the transition
// fires on it and leaves `outer` at once. Standard SysML v2 has no rule by which
// a kept signal outranks a transition of an enclosing state; a transition fires
// whenever it is enabled.
package Test {
    item def Ping;
    item def Go;

    state EnclosingTransitionWins {
        entry; then outer;
        state outer {
            entry; then waiting;
            state waiting {
                item deferred : Ping[*] ordered;
                do action buffer {
                    first start then receive;
                    action receive accept kept : Ping;
                    then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
                    then receive;
                }
                exit action flush {
                    for kept in deferred { send kept to self; }
                    then action clear { assign deferred := (); }
                }
            }
            state open;
            transition first waiting accept Go then open;
        }
        state pinged;

        transition first outer accept Ping then pinged;
    }
}
