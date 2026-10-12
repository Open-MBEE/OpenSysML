// A tree whose exposed elements relate to one another: the specializations,
// redefinitions, subsettings and typings a block definition diagram draws as
// lines between its boxes.
package Fleet {
    attribute def Mass;

    part def Wheel;

    part def Chassis {
        attribute mass : Mass;
    }

    part def LightChassis :> Chassis {
        attribute :>> mass;
    }

    part def Driver;

    part def Trailer;

    part def Vehicle {
        part chassis : Chassis;
        part wheels[4] : Wheel;
        ref part driver[0..1] : Driver;
        attribute total : Mass;
        part def Cabin;
        part cabin : Cabin;
        part radio : Radio;
        part trailer : Trailer;
    }

    part def SportsCar :> Vehicle {
        part :>> chassis : LightChassis;
        part spare : Wheel subsets wheels;
    }

    part fleetCar : Vehicle;
}

package FleetViews {
    private import Views::*;
    private import DiagramLayout::*;

    view structure {
        expose Fleet::Vehicle;
        expose Fleet::SportsCar;
        expose Fleet::Chassis;
        expose Fleet::LightChassis;
        expose Fleet::Wheel;
        expose Fleet::Driver;
        expose Fleet::Mass;
        expose Fleet::fleetCar;
    }

    // Routes the composition standing for the wheels by the usage it is
    // labelled with, the one member the edge has.
    view routed {
        expose Fleet::Vehicle;
        expose Fleet::Wheel;
        metadata Layout about Fleet::Vehicle { x = 0; y = 0; width = 200; height = 120; }
        metadata Layout about Fleet::Wheel { x = 400; y = 0; width = 120; height = 60; }
        metadata Route about Fleet::Vehicle::wheels { points = (100, 200, 300, 200); }
    }
}
