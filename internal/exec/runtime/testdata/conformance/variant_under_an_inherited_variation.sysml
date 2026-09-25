// A usage typed by a variation definition is a variation point without restating
// the modifier: its variants are choices rather than features, and each
// specializes it, so the untyped ones below get `power` from the variation's type.
package test {
    private import ScalarValues::Real;

    part def Engine { attribute power : Real = 100.0; }
    variation part def EngineChoice :> Engine;

    part def Car {
        part engine : EngineChoice {
            variant part electric { attribute :>> power = 150.0; }
            variant part petrol;
        }
        attribute curbMass : Real = 900.0 + engine.power;
    }

    part electricCar : Car {
        part :>> engine = engine::electric;

        assert constraint isElectric {
            engine == engine::electric
        }
    }
}
