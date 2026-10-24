// An automatically laid-out tree whose owned features fold into compartment
// rows of their owner on request: the definitions of a controller and its
// sensor, and a port-bearing hub whose nested ports list under their port.
package CompartmentDemo {
    part def Sensor {
        attribute label : ScalarValues::String;
        attribute reading : ScalarValues::Real;
    }
    part def Controller {
        attribute enabled : ScalarValues::Boolean;
        part sensor : Sensor;
    }
}

package PortDemo {
    private import ScalarValues::*;

    port def ClockPort { attribute rate : Real; }
    port def DataPort {
        attribute width : Natural;
        port clock : ClockPort;
    }
    part def Hub {
        attribute channels : Natural;
        port input : DataPort;
        port output : DataPort { port sync : ClockPort; }
        part buffer : Buffer;
        // A nested definition keeps its owner a node under every folding.
        part slot { part def Pin; }
    }
    part def Buffer;
}

package CompartmentDemoViews {
    private import Views::*;

    view controllers {
        expose CompartmentDemo::Sensor;
        expose CompartmentDemo::Controller;
        render asTreeDiagram;
    }

    view hub {
        expose PortDemo::Hub;
        expose PortDemo::Buffer;
        render asTreeDiagram;
    }
}
