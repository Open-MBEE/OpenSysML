// A deep history re-enters the innermost substate that was active when its
// composite state was last exited, not just the outermost one a shallow history
// would restore.
package Test {
    state DeepHistory {
        entry; then start;
        state start;
        state outer {
            state middle {
                state inner;
                state innerTwo;
            }
            deep history resume;
        }
        state away;

        succession first start then inner;

        transition first inner when Next then innerTwo;
        transition first innerTwo when Leave then away;
        transition first away when Back then resume;
        transition first resume then inner;
        transition first innerTwo when Finish then done;
    }
}
