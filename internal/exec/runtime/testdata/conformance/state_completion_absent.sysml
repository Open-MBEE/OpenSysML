// A machine no transition takes to `done` stays active, even resting in a state
// with no outgoing transition: a sink is not a completion.
package test {
    attribute def Stop;

    state sm {
        entry; then start;
        state start;
        state resting;
        transition first start accept Stop then resting;
    }
}
