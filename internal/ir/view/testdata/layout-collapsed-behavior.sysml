package FoldedBehaviors {
    private import DiagramLayout::*;
    private import Views::*;
    private import StandardViewDefinitions::*;
    state def Machine {
        entry; then idle;
        state idle;
        state running {
            entry; then warm;
            state warm;
            state hot;
            transition warm then hot;
        }
        transition idle then running;
    }
    action def Flow {
        in flowIn;
        action a { action inner1 { in y; } action inner2; first inner1 then inner2; }
        action b;
        first a then b;
        bind flowIn = a.inner1.y;
    }
    metadata Layout about Machine::running { collapsed = true; }
    metadata Layout about Flow::a { collapsed = true; }
    metadata Note about Machine::running { text = "runs until hot"; x = 40; y = 120; }

    // A collapsed Layout folds a state's and an action's body in the behaviour forms too.
    view states : StateTransitionView { expose Machine; }
    view flow : ActionFlowView { expose Flow; }
}
