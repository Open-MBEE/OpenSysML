// A calc usage nested in a definition's body, subsetting a valued calc usage,
// computes from the subsetted usage's `in` value, as one valued in place does.
package test {
    private import ScalarValues::*;
    calc def F {
        in x : Integer;
        x + 1
    }
    calc c : F {
        in x = 9;
    }
    calc def N {
        calc inner : F :> c;
        calc own : F {
            in x = 19;
        }
        inner + own
    }
    calc n : N;
}
