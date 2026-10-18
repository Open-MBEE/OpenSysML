package Drone {
    private import ScalarValues::Real;

    part def Drone {
        attribute massKilograms : Real;
    }

    requirement def MassRequirement {
        doc /* The drone shall have a mass of at most 2 kilograms. */
        subject drone : Drone;
        require constraint { drone.massKilograms <= 2.0 }
    }

    requirement droneMass : MassRequirement;
}
