package Smoke {
    private import ScalarValues::*;
    private import VerificationCases::*;

    part def SmokeDetector {
        attribute alarmLoudnessDecibels : Real;
        attribute responseTimeSeconds : Real;
    }

    requirement def LoudnessRequirement {
        doc /* The alarm shall sound at 85 decibels or louder. */
        subject detector : SmokeDetector;
        require constraint { detector.alarmLoudnessDecibels >= 85.0 }
    }

    requirement def ResponseRequirement {
        doc /* The detector shall alarm within 30 seconds of smoke. */
        subject detector : SmokeDetector;
        require constraint { detector.responseTimeSeconds <= 30.0 }
    }

    requirement def DetectorSpecification {
        subject detector : SmokeDetector;
        requirement loudness : LoudnessRequirement;
        requirement response : ResponseRequirement;
    }

    part kitchenDetector : SmokeDetector {
        attribute redefines alarmLoudnessDecibels = 90.0;
        attribute redefines responseTimeSeconds = 20.0;
    }

    requirement kitchenSpecification : DetectorSpecification;

    satisfy kitchenSpecification by kitchenDetector;

    verification def DetectorTest {
        subject detector : SmokeDetector;
        objective {
            verify kitchenSpecification;
        }
        return verdict : VerdictKind;
    }

    verification kitchenTest : DetectorTest {
        subject detector = kitchenDetector;
    }
}
