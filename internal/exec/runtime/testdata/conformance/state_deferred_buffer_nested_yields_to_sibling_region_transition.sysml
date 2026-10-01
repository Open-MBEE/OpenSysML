// A buffer nested deeper than a sibling region's transition does not hold the
// transition back either: `deep`, inside `left`'s `waiting`, keeps Ping through
// the standard deferred-signal encoding, and `right`'s transition on Ping fires
// on the same occurrence. Leaving `waiting` on Go exits `deep`, which sends the
// kept Ping back to self, where no active state accepts it any more.
package Test {
    item def Ping;
    item def Go;

    state def NestedSiblingTransitionWins parallel {
        state left {
            entry; then waiting;
            state waiting {
                entry; then deep;
                state deep {
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
            }
            state open;
            transition first waiting accept Go then open;
        }

        state right {
            entry; then idle;
            state idle;
            state pinged;
            transition first idle accept Ping then pinged;
        }
    }
}
