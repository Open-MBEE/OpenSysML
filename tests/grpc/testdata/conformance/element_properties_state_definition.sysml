// The StateDefinition metaclass family read by specification name: the states a
// definition owns, its entry action, parallelism and the transitions between
// its states.
package StateFamily {
    private import ScalarValues::*;
    attribute def Start;
    attribute def Stop;
    state def Engine {
        entry; then off;
        state off;
        state running;
        transition off_to_running first off accept Start then running;
        transition running_to_off first running accept Stop then off;
    }
}
