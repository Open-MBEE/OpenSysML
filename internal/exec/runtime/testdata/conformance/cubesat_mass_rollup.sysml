// Reduction of CubesatExample from Open-MBEE/SysML-v2-Applications-and-Examples:
// nested parts subset a feature redefined under another name, and the total mass
// rolls up over it in the unit its parts are measured in.
package test {
    private import SI::*;
    private import RealFunctions::*;

    part def Component {
        attribute mass :> ISQ::mass;
    }

    part def System :> Component {
        attribute individualmass :>> mass;
        part Subsystems : Component[*];
        attribute totalmass :> ISQ::mass = individualmass + sum(Subsystems.mass);
    }

    part def Cubesat :> System {
        part subsystems : Component[*] :>> Subsystems;
    }

    part sat : Cubesat {
        attribute :>> individualmass = 4 [kg];
        part bus : Component :> subsystems {
            attribute :>> mass = 2 [kg];
        }
        part sensor : Component :> subsystems {
            attribute :>> mass = 1 [kg];
        }
    }
}
