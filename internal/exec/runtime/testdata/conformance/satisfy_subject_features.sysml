// A requirement feature carrying no value of its own reads the satisfying
// object's feature of that name, so a requirement stated over the values it
// checks reaches a verdict from the part named by `by`. Unit-free values:
// quantity expressions such as `1.5 [m/s]` are ROADMAP A1a.
package test {
    part def Lander {
        attribute verticalSpeed;
    }

    requirement def TouchdownRequirement {
        attribute verticalSpeed;
        attribute maxVerticalSpeed;
        require constraint {
            verticalSpeed <= maxVerticalSpeed
        }
    }

    requirement touchdown : TouchdownRequirement {
        attribute :>> maxVerticalSpeed = 1.5;
    }

    part lander01 : Lander {
        attribute :>> verticalSpeed = 1.2;
    }

    part lander02 : Lander {
        attribute :>> verticalSpeed = 2.2;
    }

    part analysisContext {
        assert satisfy touchdown by lander01;
        assert satisfy touchdown by lander02;
    }
}
