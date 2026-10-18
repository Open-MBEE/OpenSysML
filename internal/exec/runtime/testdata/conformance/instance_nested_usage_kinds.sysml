// Every usage kind a definition owns is a feature of its objects: a case, use case,
// view, viewpoint, concern, rendering, enum, satisfy requirement and include use case
// usage read on the object, and the library collections of those kinds are empty.
package test {
    private import ScalarValues::*;

    part def Sub;
    requirement def Limit { subject s : Sub; }
    case def Inspection { subject s : Sub; }
    use case def Drive { subject s : Sub; }
    concern def Safety;
    viewpoint def Operator { frame concern safety : Safety; }
    view def Overview;
    rendering def Table;
    enum def Mode { on; off; }

    part def Vehicle {
        part sub : Sub;
        case inspection : Inspection;
        use case drive : Drive;
        view overview : Overview;
        viewpoint operator : Operator;
        concern safety : Safety;
        rendering table : Table;
        enum mode : Mode = Mode::on;
        satisfy requirement limit : Limit by sub;
        include use case trip : Drive;
    }
}
