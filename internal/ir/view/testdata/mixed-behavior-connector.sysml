package BehaviorConnector {
    private import OpenSysMLRenderings::*;

    part def Vehicle {
        action inspect {
            part a;
            part b;
            connect a to b;
        }
    }

    view connectorView {
        expose Vehicle;
        render asMixedDiagram;
    }
}
