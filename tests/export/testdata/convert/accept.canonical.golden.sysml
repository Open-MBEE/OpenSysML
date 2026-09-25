package Accepts {
    item def Scene;
    port def Camera;
    action def Observe {
        action trigger accept scene : Scene;
    }
    action def Watch {
        port lens : Camera;
        action framed accept scene : Scene via lens;
    }
}
