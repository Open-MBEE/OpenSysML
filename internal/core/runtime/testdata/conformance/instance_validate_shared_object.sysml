// One object two features hold is validated once, under the path it was first
// reached by, but a satisfaction stated about either feature is about it: the
// reference `spare` is bound to the composite `wheel`, and both assertions
// name the reference, one of them chaining on through it to the hub.
package test {
    private import ScalarValues::Real;

    part def Hub {
        attribute torque : Real = 90.0;
    }

    part def Wheel {
        attribute pressure : Real default = 20.0;
        part hub : Hub;
        assert constraint { pressure > 0.0 }
    }

    requirement def Inflated {
        subject w : Wheel;
        require constraint { w.pressure >= 30.0 }
    }

    requirement def Torqued {
        subject h : Hub;
        require constraint { h.torque >= 80.0 }
    }

    part def Car {
        part wheel : Wheel;
        ref part spare : Wheel = wheel;
    }

    part car : Car;
    requirement inflated : Inflated;
    requirement torqued : Torqued;
    satisfy inflated by car.spare;
    satisfy torqued by car.spare.hub;
}
