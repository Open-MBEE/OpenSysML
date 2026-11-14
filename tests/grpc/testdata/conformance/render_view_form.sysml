package FormDemo {
    private import Views::*;
    part def Wheel;
    part def Vehicle {
        part wheel : Wheel;
    }
    view structure {
        expose Vehicle;
        expose Wheel;
        render asTreeDiagram;
    }
    view parts {
        expose Vehicle;
        render asElementTable;
    }
}
