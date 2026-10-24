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
        action a { action inner1; action inner2; first inner1 then inner2; }
        action b;
        first a then b;
    }
    metadata Layout about Machine::running { collapsed = true; }
    metadata Layout about Flow::a { collapsed = true; }

    // A collapsed Layout folds a state's and an action's body in the behaviour forms too.
    view states : StateTransitionView { expose Machine; }
    view flow : ActionFlowView { expose Flow; }
}
