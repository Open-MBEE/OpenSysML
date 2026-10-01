// A state keeps a signal it cannot handle through the standard SysML v2
// encoding of a deferred signal: an ordered buffer the do action's accept loop
// fills while `busy` is active, which the exit action sends back to self. The
// Ping arriving while `busy` is active is retained rather than dropped, and is
// delivered once the machine has left `busy` for a state that handles it.
package DeferredSignalKeptAndReplayed {
    item def Ping;
    item def Go;

    state Panel {
        entry; then start;
        state start;
        state busy {
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
        state ready;

        succession first start then busy;

        transition first busy accept Go then ready;
        transition first ready accept Ping then done;
    }
}
