package test {
    private import ScalarValues::*;
    view def Overview {
        attribute depth : Integer = 1;
        assert constraint shallow { depth < 1 }
    }
    case def Inspection {
        attribute passes : Integer = 2;
        assert constraint repeated { passes > 1 }
    }
    requirement def Safe;
    part def Vehicle {
        view overview : Overview;
        case inspection : Inspection;
        view sketch;
        case audit;
        use case drive;
        viewpoint ownerView;
        concern cost;
        rendering table;
        satisfy requirement safe : Safe;
    }
    part v : Vehicle;
}
