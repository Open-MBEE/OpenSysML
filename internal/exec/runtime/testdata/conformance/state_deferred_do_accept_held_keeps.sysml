// A state deferring Ping whose own do behavior accepts Ping at an action that
// also awaits another branch: while `take` is held for its second arrival it
// cannot take the occurrence, so the keeping loop keeps the Ping rather than
// leaving it in flight for an accept that cannot run. Arm releases `take`,
// which finds no Ping, so `heard` stays 0; the kept Ping is replayed when
// `busy` exits and `ready` takes it.
package Test {
    private import ScalarValues::Integer;
    item def Ping;
    item def Go;
    item def Arm;

    state DoAcceptHeldKeeps {
        attribute heard : Integer = 0;
        entry; then start;
        state start;
        state busy {
            @MigrationMetadata::DeferredEvent { ref :>> signal : Ping; }
            item deferred : Ping[*] ordered;
            do action buffer {
                first start then split;
                fork split;
                then receive;
                then work;
                #MigrationMetadata::DeferredKeeper action receive accept kept : Ping;
                then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
                then receive;
                action work {
                    first start then gate;
                    fork gate;
                    then arm;
                    then take;
                    action arm accept a : Arm;
                    then take;
                    action take accept p : Ping;
                    then action count { assign heard := heard + 1; }
                }
            }
            exit action flush {
                for kept in deferred { send kept to self; }
                then action clear { assign deferred := (); }
            }
        }
        state ready;
        succession first start then busy;
        transition first busy accept Go then ready;
        transition first ready accept Ping then done;
    }
}
