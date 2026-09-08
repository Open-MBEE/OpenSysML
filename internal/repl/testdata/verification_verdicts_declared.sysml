package Landing {
    private import ScalarValues::*;

    part def Lander {
        attribute verticalSpeed : Real default = 1.2;
    }

    part slowLander : Lander;

    assert satisfy requirement grounded by slowLander {
        subject lander : Lander;
        require constraint {
            lander.verticalSpeed <= 1.5
        }
    }

    verification def SpeedCheck {
        subject lander : Lander;
        objective { verify grounded; }
        VerificationCases::PassIf(lander.verticalSpeed <= 1.5)
    }

    verification checkSlow : SpeedCheck {
        subject lander = slowLander;
    }
}
