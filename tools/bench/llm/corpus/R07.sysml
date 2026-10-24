package Pacemaker {
    private import ScalarValues::*;

    part def Pacemaker {
        attribute batteryLifeYears : Real;
        attribute minimumRateBeatsPerMinute : Integer;
    }

    requirement def BatteryLifeRequirement {
        doc /* The pacemaker battery shall last at least 7 years. */
        subject device : Pacemaker;
        require constraint { device.batteryLifeYears >= 7.0 }
    }

    requirement def MinimumRateRequirement {
        doc /* The pacemaker shall maintain at least 60 beats per minute. */
        subject device : Pacemaker;
        require constraint { device.minimumRateBeatsPerMinute >= 60 }
    }

    requirement def PacemakerSpecification {
        subject device : Pacemaker;
        requirement batteryLife : BatteryLifeRequirement;
        requirement minimumRate : MinimumRateRequirement;
    }

    part modelA : Pacemaker {
        attribute redefines batteryLifeYears = 10.0;
        attribute redefines minimumRateBeatsPerMinute = 60;
    }

    requirement modelASpecification : PacemakerSpecification;

    satisfy modelASpecification by modelA;
}
