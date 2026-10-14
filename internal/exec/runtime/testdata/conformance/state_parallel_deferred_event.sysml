// A parallel state owns a DeferredEvent annotation and the migrator's standard
// deferred-signal encoding. Ping is kept while Busy is active and replayed to
// Ready after Leave exits Busy.
package ParallelDeferredEvent {
    item def Ping;
    item def Leave;

    state def Machine {
        entry; then start;
        state start;
        state Busy parallel {
            @MigrationMetadata::DeferredEvent { ref :>> signal : Ping; }
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
            state R {
                entry; then active;
                state active;
            }
        }
        state Ready;

        succession first start then Busy;
        transition first Busy accept Leave then Ready;
        transition first Ready accept Ping then done;
    }
}
