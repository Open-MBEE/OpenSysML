// An exit whose input is optional runs on every leaving transition: a firing
// carrying the payload binds it, a data-less one leaves it empty, and only the
// statements reading it are skipped then.
package StateExitPayloadOptionalInput {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    attribute def Warning :> Integer;
    attribute def Reset;

    state Monitor {
        attribute exits : Integer = 0;
        attribute levels : Integer = 0;

        entry; then idle;
        state idle {
            exit action {
                in level : Integer[0..1] = warn.w;
                assign exits := exits + 1;
                if notEmpty(level) {
                    assign levels := levels * 10 + level;
                }
            }
        }
        state warned;

        transition warn first idle accept w : Warning then warned;
        transition reset first idle accept Reset then warned;
        transition first warned accept Reset then idle;
    }
}
