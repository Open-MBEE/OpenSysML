// The conditions a satisfaction assertion evaluates include the ones the
// satisfied requirement inherits from the requirement definition it specializes,
// and the values the requirement usage binds mask the defaults they redefine.
package test {
    part def Lander {
        attribute verticalSpeed;
        attribute horizontalSpeed;
    }

    requirement def VerticalTouchdown {
        subject lander : Lander;
        attribute maxVerticalSpeed = 3.0;
        require constraint {
            lander.verticalSpeed <= maxVerticalSpeed
        }
    }

    requirement def SafeTouchdown :> VerticalTouchdown {
        attribute maxHorizontalSpeed;
        require constraint {
            lander.horizontalSpeed <= maxHorizontalSpeed
        }
    }

    requirement touchdown : SafeTouchdown {
        attribute :>> maxVerticalSpeed = 1.5;
        attribute :>> maxHorizontalSpeed = 0.5;
    }

    part driftingLander : Lander {
        attribute :>> verticalSpeed = 1.2;
        attribute :>> horizontalSpeed = 0.9;
    }

    part divingLander : Lander {
        attribute :>> verticalSpeed = 2.4;
        attribute :>> horizontalSpeed = 0.1;
    }

    part gentleLander : Lander {
        attribute :>> verticalSpeed = 1.2;
        attribute :>> horizontalSpeed = 0.1;
    }

    part analysisContext {
        assert satisfy touchdown by driftingLander;
        assert satisfy touchdown by divingLander;
        assert satisfy touchdown by gentleLander;
    }
}
