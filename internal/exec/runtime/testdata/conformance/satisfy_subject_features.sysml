// A requirement stated over its own features is given the satisfying object's
// values by a binding, not by matching names (SysML v2 §7.21.1): the usage states
// its subject and redefines the feature to be the subject's, so the part named by
// `by` is the subject the feature reads. Unit-free values: quantity expressions
// such as `1.5 [m/s]` are ROADMAP A1a.
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
        subject lander : Lander;
        attribute :>> verticalSpeed = lander.verticalSpeed;
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
