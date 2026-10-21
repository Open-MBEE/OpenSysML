// The pins of a state's entry, do and exit actions take the multiplicity rule too: each is
// a performed action, so a [1] pin reads bare and a bare pin reads as a one-element
// sequence, and what flows out through an inout pin onto the machine's features keeps the
// shape those features' own multiplicities give it.
package test {
    private import ScalarValues::*;

    item def Go;

    state machine {
        action def Note {
            in value : Integer;
            inout seen : Integer;
            inout seenOne : Integer[1];
            first start;
            then action s { assign seen := value; assign seenOne := value; }
            then done;
        }

        attribute level : Integer := 3;
        out entered : Integer = 0;
        out enteredOne : Integer[1] = 0;
        out worked : Integer = 0;
        out workedOne : Integer[1] = 0;
        out left : Integer = 0;
        out leftOne : Integer[1] = 0;

        entry; then active;
        state active {
            entry action e : Note { in value = level; inout seen = entered; inout seenOne = enteredOne; }
            do action d : Note { in value = level * 2; inout seen = worked; inout seenOne = workedOne; }
            exit action x : Note { in value = level * 3; inout seen = left; inout seenOne = leftOne; }
        }
        state finished;
        transition first active accept Go then finished;
    }
}
