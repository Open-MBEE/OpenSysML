// A requirement over its own features gets the satisfying object's values by a stated binding to its
// subject, not by matching names (SysML v2 §7.21.1). Unit-free: quantity expressions are ROADMAP A1a.
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
