package Satellite {
    private import ScalarValues::Real;

    part def SolarArray {
        attribute areaSquareMeters : Real;
    }
    part def Battery {
        attribute capacityWattHours : Real;
    }
    part def Antenna;
    part def Thruster;

    part def PowerSubsystem {
        part arrays : SolarArray[2];
        part battery : Battery;
    }

    part def CommunicationSubsystem {
        part highGainAntenna : Antenna;
        part lowGainAntenna : Antenna;
    }

    part def PropulsionSubsystem {
        part thrusters : Thruster[4];
    }

    part def Satellite {
        part power : PowerSubsystem;
        part comms : CommunicationSubsystem;
        part propulsion : PropulsionSubsystem;
    }
}
