package Shipping {
    part def Hull;
    part def Engine;
    part def Bridge;

    part def Ship {
        part hull : Hull;
        part engines : Engine[2];
        part bridge : Bridge;
    }

    view def StructureView;

    view shipStructure : StructureView {
        expose Ship::**;
    }
}
