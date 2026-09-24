package Structure {
    part def Vehicle {
        part spare : Wheel;
        private attribute serial : ScalarValues::String;
        view 'Vehicle Internals' {
            expose Vehicle;
            expose serial;
            render $::Views::asInterconnectionDiagram;
        }
    }
    part def Wheel;
}
package Views {
    view 'Vehicle Overview' {
        expose Structure::Vehicle;
        expose Structure::Vehicle::spare;
        render $::Views::asTreeDiagram;
    }
}
