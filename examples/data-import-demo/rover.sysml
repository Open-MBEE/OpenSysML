// A rover fleet whose values come from outside the model: a mass-properties
// spreadsheet, a battery test rig's JSON report, motor telemetry as JSON Lines
// and a tab-separated list of overrides. See README.md for the imports.
package RoverDemo {
    private import ScalarValues::*;
    private import ISQ::*;
    private import SI::*;

    enum def Health { enum nominal; enum degraded; enum failed; }

    part def Battery {
        attribute capacity : EnergyValue;
        attribute cells : Integer;
    }

    part def Motor {
        attribute peakPower : PowerValue;
    }

    part def Rover {
        attribute serial : String;
        attribute mass : MassValue;
        attribute health : Health default = Health::nominal;
        attribute qualified : Boolean default = false;
        part battery : Battery;
        part driveMotor : Motor;
        attribute specificEnergy = battery.capacity / mass;
    }

    part scout : Rover;
    part hauler : Rover {
        attribute :>> serial = "H-002";
    }
    alias scout2 for scout;
}
