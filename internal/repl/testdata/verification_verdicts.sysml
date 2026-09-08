package Landing {
    private import ScalarValues::*;

    part def Lander {
        attribute verticalSpeed : Real default = 1.2;
    }

    requirement def TouchdownRequirement {
        subject lander : Lander;
        attribute maxVerticalSpeed;
        require constraint {
            lander.verticalSpeed <= maxVerticalSpeed
        }
    }

    requirement touchdown : TouchdownRequirement {
        subject lander = slowLander;
        attribute :>> maxVerticalSpeed = 1.5;
    }

    part slowLander : Lander;

    part fastLander : Lander {
        attribute :>> verticalSpeed = 2.4;
    }

    verification def SpeedCheck {
        subject lander : Lander;
        objective { verify touchdown; }
        VerificationCases::PassIf(lander.verticalSpeed <= 1.5)
    }

    verification checkSlow : SpeedCheck {
        subject lander = slowLander;
    }

    verification checkFast : SpeedCheck {
        subject lander = fastLander;
    }

    verification def UnreportedCheck {
        subject lander : Lander;
        objective { verify touchdown; }
        action measure;
    }

    verification silentCheck : UnreportedCheck {
        subject lander = slowLander;
    }

    verification def CheckedPlan {
        subject lander : Lander;
        verification checkOne : SpeedCheck {
            subject lander = fastLander;
        }
        return verdict : VerificationCases::VerdictKind = VerificationCases::VerdictKind::pass;
    }

    verification checkedPlan : CheckedPlan {
        subject lander = slowLander;
    }

    part analysisContext {
        assert satisfy touchdown by slowLander;
    }
}
