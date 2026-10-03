// A do body that writes, waits on the clock, then writes again: a run to
// quiescence moves the clock to the wait and runs the body to its end.
package test {
    private import ScalarValues::*;

    state sm {
        attribute c : Integer = 0;

        entry; then s;
        state s {
            do action {
                assign c := 1;
                then accept after 2 [SI::s];
                then assign c := 2;
            }
        }
    }
}
