// A shallow history re-enters the substate of its composite state that was
// active when that state was last exited, rather than the default one its
// history transition names.
package Test {
    private import StateMachines::*;
    state ShallowHistory {
        entry; then start;
        state start;
        state outer {
            state first;
            state second;
            #shallowHistory state resume;
        }
        state away;

        succession first start then first;

        transition first first when Next then second;
        transition first second when Leave then away;
        transition first away when Back then resume;
        transition first resume then first;
        transition first second when Finish then done;
    }
}
