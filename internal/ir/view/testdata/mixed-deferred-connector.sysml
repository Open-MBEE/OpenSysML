package DeferredConnector {
    private import OpenSysMLRenderings::*;

    requirement def R;

    part def Vehicle {
        part a;
        requirement r : R {
            part b;
        }
        connect a to r.b;
    }

    view connectorView {
        expose Vehicle;
        render asMixedDiagram;
    }
}
