// out and inout pins take the same rule as in pins: a [1] pin holds its value bare, a bare
// one holds a one-element sequence, on the performance and on the part performing it.
package test {
    private import ScalarValues::*;

    action def Adjust {
        inout tally : Integer;
        inout exact : Integer[1];
        out note : Integer;
        out noteExact : Integer[1];
        first start;
        then action t { assign tally := tally + 1; assign exact := exact + 1; }
        then action n { assign note := tally; assign noteExact := exact; }
        then done;
    }

    part def Counter {
        attribute base : Integer = 10;
        perform action adjust : Adjust { inout tally = base; inout exact = base; }
    }

    part dev : Counter;

    action probe {
        out tally : Integer;
        out exact : Integer[1];
        out note : Integer;
        out noteExact : Integer[1];
        first start;
        then action a : Adjust { inout tally = 10; inout exact = 10; }
        then action read {
            assign tally := a.tally;
            assign exact := a.exact;
            assign note := a.note;
            assign noteExact := a.noteExact;
        }
        then done;
    }
}
