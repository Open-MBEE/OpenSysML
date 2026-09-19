// A completion transition out of a composite state into its own shallow history
// reads the record the completion writes: `outer` completing from `second`
// leaves an empty history, so the history's default `then third` is taken after
// `outer`'s entry behavior, and `outer`'s `entry; then one` is not. Leaving
// `outer` from `one` on Leave and coming back restores `one` as before.
package Test {
    state ShallowHistoryCompletionDefault {
        attribute log : String = "";
        entry; then start;
        state start;
        state outer {
            entry action { assign log := log + "outer(entry);"; }
            exit action { assign log := log + "outer(exit);"; }
            entry; then one;
            state one {
                entry action { assign log := log + "one(entry);"; }
            }
            state second;
            state third {
                entry action { assign log := log + "third(entry);"; }
            }
            history resume;
            transition first resume do { assign log := log + "default(effect);"; } then third;
            transition first one when Next then second;
            transition first second when Finish then done;
        }
        state away;

        transition first start when Go then outer;
        transition first one when Leave then away;
        transition first away when Back then resume;
        transition first outer do { assign log := log + "completion(effect);"; } then resume;
        transition first third when End then done;
    }
}
