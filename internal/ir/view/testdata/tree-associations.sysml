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
        expose Fleet::WheelToCar;
        expose Fleet::Tows;
        expose Fleet::Pairing;
        expose Fleet::Ferries;
        render Views::asTreeDiagram;
    }
}
