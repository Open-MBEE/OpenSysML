package Scale {
    private import ScalarValues::*;
    private import VerificationCases::*;

    part def Scale {
        attribute measurementErrorGrams : Real;
    }

    requirement def AccuracyRequirement {
        doc /* The scale shall weigh to within 5 grams. */
        subject scale : Scale;
        require constraint { scale.measurementErrorGrams <= 5.0 }
    }

    verification def AccuracyTest {
        subject scale : Scale;
        objective {
            verify requirement accuracy : AccuracyRequirement;
        }
        return verdict : VerdictKind;
    }

    verification accuracyTest : AccuracyTest;
}
