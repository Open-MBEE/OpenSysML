// A transition in a sibling region wins over a buffer keeping the same signal:
// while `waiting` keeps Ping through the standard deferred-signal encoding, the
// `right` region's transition on Ping fires on the same occurrence, which every
// orthogonal region is offered. Standard SysML v2 has no rule by which a kept
// signal holds back a sibling region's transition. Leaving `waiting` on Go then
// sends the kept Ping back to self, where no active state accepts it any more.
package Test {
    item def Ping;
    item def Go;

    state def SiblingTransitionWins parallel {
        state left {
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

        state right {
            entry; then idle;
            state idle;
            state pinged;
            transition first idle accept Ping then pinged;
        }
    }
}
