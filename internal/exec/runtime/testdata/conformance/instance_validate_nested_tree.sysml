// Validating an object as a whole checks every assertion about it and about the
// objects it holds, to any depth: the asserted constraints of each object's type,
// the requirement usages it carries, and the satisfaction assertions whose
// subject is one of those objects. Each verdict names the object it is about by
// the path walked to it, a collection element indexed. Here the car's own
// assertion holds, the engine's fails at depth two, the collection wheel that
// redefines its pressure fails at depth two, the injector at depth three holds,
// and the satisfaction stated about the engine — a nested subject — holds.
package test {
    private import ScalarValues::Real;

    part def Injector {
        attribute rate : Real = 2.0;
        assert constraint ratePositive { rate > 0.0 }
    }

    part def Engine {
        attribute power : Real = 300.0;
        part injector : Injector;
        assert constraint { power < 200.0 }
    }

    part def Wheel {
        attribute pressure : Real default = 32.0;
        assert constraint pressureOk { pressure >= 30.0 }
    }

    part def Car {
        attribute mass : Real = 1500.0;
        part engine : Engine;
        part wheels : Wheel[3] {
            attribute :>> pressure = 20.0;
        }
        assert constraint massOk { mass < 2000.0 }
        requirement lightEnough {
            attribute m : Real = mass;
            require constraint { m < 1600.0 }
        }
    }

    requirement def PowerReq {
        subject e : Engine;
        require constraint { e.power > 100.0 }
    }

    part car : Car;
    requirement strongEngine : PowerReq;
    satisfy strongEngine by car.engine;
}
