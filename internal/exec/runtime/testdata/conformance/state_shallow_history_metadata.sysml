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

        succession first start then outer.first;

        transition first outer.first when Next then outer.second;
        transition first outer.second when Leave then away;
        transition first away when Back then outer.resume;
        transition first outer.resume then outer.first;
        transition first outer.second when Finish then done;
    }
}
