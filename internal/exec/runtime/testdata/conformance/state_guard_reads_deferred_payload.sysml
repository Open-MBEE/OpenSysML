// A signal kept through the standard deferred-signal encoding (an ordered
// buffer the do action's accept loop fills, which the exit action sends back to
// self) is dispatched once a state that handles it is active, and the guard of
// the transition it then fires reads its payload through the transition's name,
// `raise.l.level`, as it does for an occurrence never kept.
package StateGuardReadsDeferredPayload {
    private import ScalarValues::*;

    attribute def Level { attribute level : Integer; }
    attribute def Go;

    state Machine {
        attribute chosen : Integer = 0;

        entry; then busy;
        state busy {
            attribute deferred : Level[*] ordered;
            do action buffer {
                first start then receive;
                action receive accept kept : Level;
                then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
                then receive;
            }
            exit action flush {
                for kept in deferred { send kept to self; }
                then action clear { assign deferred := (); }
            }
        }
        state ready;
        state high;
        state low;

        transition first busy accept Go then ready;
        transition raise first ready accept l : Level if raise.l.level > 5 do assign chosen := raise.l.level then high;
        transition drop first ready accept l : Level if drop.l.level <= 5 then low;
        transition first high then done;
        transition first low then done;
    }
}
