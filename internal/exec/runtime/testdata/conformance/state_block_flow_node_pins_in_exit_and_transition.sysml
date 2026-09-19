// Action nodes in an entry, exit and transition body each perform in a frame of
// their own: `p` reads the enclosing state's `local`, `bind` returns each node's
// output to a state attribute, and the transition body reads `r.v`.
package test {
    private import ScalarValues::*;
    state def Pins {
        attribute seen : Integer = 5;
        attribute total : Integer = 0;
        attribute exited : Integer = 0;
        attribute moved : Integer = 0;
        entry; then init;
        state init;
        state active {
            attribute local : Integer = 7;
            entry action {
                if seen > 0 {
                    action p { out v : Integer; assign v := seen + local; }
                    bind p.v = total;
                }
            }
            exit action {
                if total > 0 {
                    action q { out v : Integer; assign v := total * 2; }
                    bind q.v = exited;
                }
            }
        }
        succession first init then active;
        transition active_done first active do action { action r { out v : Integer; assign v := exited + 1; } assign moved := r.v; } then done;
    }
}
