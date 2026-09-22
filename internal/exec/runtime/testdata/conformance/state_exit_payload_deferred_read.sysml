// A state's exit action reads the leaving transition's payload, `raise.l`, from
// expressions evaluated later than they are written: a body closed over the exit's
// environment and an `expr` argument the callee evaluates.
package StateExitPayloadDeferredRead {
    private import ScalarValues::*;
    private import ControlFunctions::*;

    attribute def Level :> Integer;

    state Monitor {
        attribute collected : Integer = 0;
        attribute chosen : Integer = 0;

        entry; then idle;
        state idle {
            exit action {
                in viaBody : Integer = ((10)->collect { in x : Integer; x + raise.l })#(1);
                in viaExpr : Integer = 'if'(true, raise.l * 2, 0);
                assign collected := viaBody;
                assign chosen := viaExpr;
            }
        }
        state alerting;

        transition raise first idle accept l : Level then alerting;
        transition first alerting then done;
    }
}
