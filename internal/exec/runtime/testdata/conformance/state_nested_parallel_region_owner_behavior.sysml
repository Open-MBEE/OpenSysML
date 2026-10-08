// The do behavior of a region-owning state inside a parallel state runs once
// the region is entered: `a` counts its work, then keeps the Ping arriving while
// `outer` is active through the standard deferred-signal encoding, and its exit
// action sends the kept Ping back to self once Leave moves the machine on to a
// state that accepts it.
package NestedParallelRegionOwner {
    private import ScalarValues::*;

    item def Ping;
    item def Leave;

    state def Machine {
        attribute worked : Integer = 0;

        entry; then start;
        state start;
        state outer parallel {
            state a {
                item deferred : Ping[*] ordered;
                do action work {
                    first start;
                    then action count { assign worked := worked + 1; }
                    then receive;
                    action receive accept kept : Ping;
                    then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
                    then receive;
                }
                exit action flush {
                    for kept in deferred { send kept to self; }
                    then action clear { assign deferred := (); }
                }
                entry; then a1;
                state a1;
            }
            state b {
                entry; then b1;
                state b1;
            }
        }
        state waited;

        succession first start then outer;
        transition first outer when Leave then waited;
        transition first waited when Ping then done;
    }
}
