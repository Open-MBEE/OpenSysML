// `assert not satisfy` asserts that the requirement does not hold of the
// subject, so its verdict is the inverse of the satisfaction's.
package test {
    part def Lander {
        attribute verticalSpeed;
    }

    requirement def TouchdownRequirement {
        subject lander : Lander;
        attribute maxVerticalSpeed;
        require constraint {
            lander.verticalSpeed <= maxVerticalSpeed
        }
    }

    requirement touchdown : TouchdownRequirement {
        attribute :>> maxVerticalSpeed = 1.5;
    }

    part slowLander : Lander {
        attribute :>> verticalSpeed = 1.2;
    }

    part fastLander : Lander {
        attribute :>> verticalSpeed = 2.4;
    }

    part analysisContext {
        assert not satisfy touchdown by fastLander;
        assert not satisfy touchdown by slowLander;
    }
}
