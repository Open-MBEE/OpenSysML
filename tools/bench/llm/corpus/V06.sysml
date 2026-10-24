package Aircraft {
    part def Wing;
    part def Fuselage;
    part def Engine;

    part def Aircraft {
        doc /* A twin-engine passenger aircraft. */
        part leftWing : Wing;
        part rightWing : Wing;
        part fuselage : Fuselage;
        part engines : Engine[2];
    }

    view def StructuralView {
        doc /* Shows the physical decomposition of a system. */
    }

    rendering def TreeDiagram;
    rendering asTree : TreeDiagram;

    view aircraftStructure : StructuralView {
        expose Aircraft::**;
        render asTree;
    }
}
