// A transition from a substate into the composite state enclosing it completes
// the composite's body, so the composite's history keeps no substate: leaving
// the composite afterwards and re-entering it through its history performs the
// history's default transition, not the substate the transition exited.
package Test {
    state TransitionIntoActiveAncestorHistory {
        attribute log : String = "";
        entry; then outer;
        state outer {
            entry action { assign log := log + "outer(entry);"; }
            exit action { assign log := log + "outer(exit);"; }
            history resume;
            entry; then start;
            state start {
                entry action { assign log := log + "start(entry);"; }
                exit action { assign log := log + "start(exit);"; }
            }
            state inner {
                entry action { assign log := log + "inner(entry);"; }
                exit action { assign log := log + "inner(exit);"; }
            }
            transition first start accept Go then inner;
            transition first inner accept Up do { assign log := log + "up(effect);"; } then outer;
            transition first resume do { assign log := log + "default(effect);"; } then start;
        }
        state away {
            entry action { assign log := log + "away(entry);"; }
        }
        transition first outer accept Leave do { assign log := log + "leave(effect);"; } then away;
        transition first away accept Back do { assign log := log + "back(effect);"; } then resume;
        transition first outer accept Finish then done;
    }
    attribute def Go;
    attribute def Up;
    attribute def Leave;
    attribute def Back;
    attribute def Finish;
}
