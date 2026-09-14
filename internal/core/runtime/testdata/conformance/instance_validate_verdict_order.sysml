// Verdicts go out object by object, root first and then each held object as the
// walk reaches it, whatever kind of assertion each is: the satisfactions about
// the root — one its type carries, one stated in the scope — precede the
// constraint asserted on the engine it holds, and the one about the engine follows it.
package test {
    private import ScalarValues::Real;

    part def Engine {
        attribute power : Real = 300.0;
        assert constraint powered { power > 0.0 }
    }

    requirement def Light {
        subject c : Car;
        require constraint { c.mass < 2000.0 }
    }

    requirement def Strong {
        subject e : Engine;
        require constraint { e.power > 100.0 }
    }

    part def Car {
        attribute mass : Real = 1500.0;
        part engine : Engine;
        requirement light {
            require constraint { mass < 1600.0 }
        }
        assert satisfy light;
    }

    part car : Car;
    requirement strong : Strong;
    requirement alsoLight : Light;
    satisfy alsoLight by car;
    satisfy strong by car.engine;
}
