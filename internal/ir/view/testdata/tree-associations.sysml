// A tree whose exposed elements include connection defs: the associations a
// block definition diagram draws as lines between the boxes of their end
// types, and the ones it keeps as boxes.
package Fleet {
    part def Car {
        ref part wheels : Wheel[4];
    }

    part def Wheel {
        ref part car : Car;
    }

    connection def WheelToCar {
        end wheels : Wheel crosses car.wheels;
        end car : Car crosses wheels.car;
    }

    part def Hitch;

    part def Truck {
        part hitch : Hitch;
    }

    connection def HitchToTruck {
        end hitch : Hitch crosses truck.hitch;
        end truck : Truck;
    }

    part def Trailer;

    connection def Tows {
        end tower : Car;
        end [0..1] ref trailer : Trailer;
    }

    connection def Pairing {
        end a : Car;
        end b : Trailer;
        attribute tag : ScalarValues::String;
    }

    connection def Keyed {
        end a : Car {
            attribute key : ScalarValues::String;
        }
        end b : Trailer;
    }

    connection def Towing :> Tows;

    part def Boat;

    connection def Ferries {
        end boat : Boat;
        end car : Car;
    }
}

package FleetViews {
    view structure {
        expose Fleet::Car;
        expose Fleet::Wheel;
        expose Fleet::Trailer;
        expose Fleet::Truck;
        expose Fleet::Hitch;
        expose Fleet::WheelToCar;
        expose Fleet::HitchToTruck;
        expose Fleet::Tows;
        expose Fleet::Pairing;
        expose Fleet::Ferries;
        expose Fleet::Keyed;
        expose Fleet::Towing;
        render Views::asTreeDiagram;
    }
}
