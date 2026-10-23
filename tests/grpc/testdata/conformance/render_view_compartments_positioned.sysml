package CompartmentDemo {
    private import ScalarValues::*;
    private import Views::*;
    part def Sensor {
        attribute label : String;
        attribute reading : Real;
    }
    part def Controller {
        attribute enabled : Boolean;
        part sensor : Sensor;
    }
    view structure {
        expose Sensor;
        expose Controller;
        render asTreeDiagram;
    }
}
