package Insulin {
    private import ScalarValues::*;
    private import VerificationCases::*;

    part def InsulinPump {
        attribute doseAccuracyPercent : Real;
        attribute occlusionAlarmSeconds : Real;
        attribute batteryLifeHours : Real;
    }

    requirement def DoseAccuracyRequirement {
        doc /* Delivered dose shall be within 5 percent of the programmed dose. */
        subject pump : InsulinPump;
        require constraint { pump.doseAccuracyPercent <= 5.0 }
    }

    requirement def OcclusionAlarmRequirement {
        doc /* An occlusion shall raise an alarm within 120 seconds. */
        subject pump : InsulinPump;
        require constraint { pump.occlusionAlarmSeconds <= 120.0 }
    }

    requirement def BatteryLifeRequirement {
        doc /* The pump shall run for at least 72 hours on one battery. */
        subject pump : InsulinPump;
        require constraint { pump.batteryLifeHours >= 72.0 }
    }

    requirement def SafetyRequirements {
        subject pump : InsulinPump;
        requirement doseAccuracy : DoseAccuracyRequirement;
        requirement occlusionAlarm : OcclusionAlarmRequirement;
    }

    requirement def PumpSpecification {
        subject pump : InsulinPump;
        requirement safety : SafetyRequirements;
        requirement batteryLife : BatteryLifeRequirement;
    }

    part pumpModelX : InsulinPump {
        attribute redefines doseAccuracyPercent = 3.0;
        attribute redefines occlusionAlarmSeconds = 60.0;
        attribute redefines batteryLifeHours = 96.0;
    }

    requirement modelXSpecification : PumpSpecification;

    satisfy modelXSpecification by pumpModelX;

    verification def BenchTest {
        subject pump : InsulinPump;
        objective {
            verify modelXSpecification;
        }
        action measureDose;
        then action simulateOcclusion;
        then action dischargeBattery;
        return verdict : VerdictKind;
    }

    verification modelXBenchTest : BenchTest {
        subject pump = pumpModelX;
    }
}
