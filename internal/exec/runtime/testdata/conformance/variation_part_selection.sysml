// A variation part is bound to one of its variant parts, and the nested values
// the selected variant declares are read through the variation feature.
package test {
    private import ScalarValues::Real;

    part def Engine {
        attribute power : Real;
        attribute mass : Real;
    }

    part def Vehicle {
        part engine : Engine;
        attribute curbMass : Real;
    }

    abstract part vehicleFamily : Vehicle {
        attribute :>> curbMass = 900.0 + engine.mass;

        variation part :>> engine {
            variant part electric : Engine {
                attribute :>> power = 150.0;
                attribute :>> mass = 300.0;
            }
            variant part petrol : Engine {
                attribute :>> power = 120.0;
                attribute :>> mass = 200.0;
            }
        }
    }

    part electricVehicle :> vehicleFamily {
        part :>> engine = engine::electric;

        assert constraint isElectric {
            engine == engine::electric
        }
        assert constraint enoughPower {
            engine.power >= 150.0
        }
    }
}
