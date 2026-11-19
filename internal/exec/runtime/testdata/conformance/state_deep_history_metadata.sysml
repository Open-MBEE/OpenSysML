// A deep history re-enters the innermost substate that was active when its
// composite state was last exited, not just the outermost one a shallow history
// would restore.
package Test {
    private import StateMachines::*;
    state DeepHistory {
        entry; then start;
        state start;
        state outer {
            state middle {
                state inner;
                state innerTwo;
            }
            #deepHistory state resume;
        }
        state away;

        succession first start then outer.middle.inner;

        transition first outer.middle.inner when Next then outer.middle.innerTwo;
        transition first outer.middle.innerTwo when Leave then away;
        transition first away when Back then outer.resume;
        transition first outer.resume then outer.middle.inner;
        transition first outer.middle.innerTwo when Finish then done;
    }
}
