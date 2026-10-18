package Factory {
    private import StandardViewDefinitions::*;

    part def Press;
    part def Conveyor;
    part def Robot;

    part def Line {
        part press : Press;
        part conveyor : Conveyor;
        part robots : Robot[2];
    }

    view lineTree : BrowserView {
        expose Line::**;
    }
}
