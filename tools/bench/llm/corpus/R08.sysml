package Parachute {
    private import ScalarValues::*;
    private import VerificationCases::*;

    part def Parachute {
        attribute descentRateMetresPerSecond : Real;
        attribute deploymentTimeSeconds : Real;
    }

    requirement def DescentRateRequirement {
        doc /* The parachute shall limit descent to 5 metres per second. */
        subject chute : Parachute;
        require constraint { chute.descentRateMetresPerSecond <= 5.0 }
    }

    requirement def DeploymentTimeRequirement {
        doc /* The parachute shall fully deploy within 3 seconds. */
        subject chute : Parachute;
        require constraint { chute.deploymentTimeSeconds <= 3.0 }
    }

    verification def DropTest {
        subject chute : Parachute;
        objective {
            verify requirement descentRate : DescentRateRequirement;
            verify requirement deploymentTime : DeploymentTimeRequirement;
        }
        action measureDescent;
        then action measureDeployment;
        return verdict : VerdictKind;
    }

    verification dropTest : DropTest;
}
