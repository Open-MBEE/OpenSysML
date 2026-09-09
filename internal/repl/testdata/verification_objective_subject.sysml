package Landing {
    private import ScalarValues::*;

    part def Lander {
        attribute touchdownSpeed : Real;
    }

    part scout : Lander {
        attribute :>> touchdownSpeed = 1.2;
    }

    part heavy : Lander {
        attribute :>> touchdownSpeed = 1.6;
    }

    requirement def SoftLanding {
        subject lander : Lander;
        in attribute limit : Real default = 1.5;
        require constraint {
            lander.touchdownSpeed <= limit
        }
    }

    requirement softLanding : SoftLanding {
        subject lander = scout;
    }

    verification def TouchdownCheck {
        subject lander : Lander;
        in attribute limit : Real = 1.5;
        objective : SoftLanding {
            in limit = limit;
            verify softLanding;
        }
        VerificationCases::PassIf(lander.touchdownSpeed <= limit)
    }

    verification checkScout : TouchdownCheck {
        subject lander = scout;
    }

    verification checkHeavy : TouchdownCheck {
        subject lander = heavy;
    }

    part analysisContext {
        assert satisfy softLanding by scout;
    }
}
