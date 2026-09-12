// A launch vehicle whose total mass is a recursive rollup over its parts.
// Only instantiating the stack tells you whether the total can be computed.
package MassRollup {
    private import ISQ::*;
    private import SI::*;
    private import RealFunctions::sum;

    abstract part def Component {
        part subcomponents : Component[*] default null;
        attribute mass :> ISQ::mass;
        attribute totalMass :> ISQ::mass default = mass + sum(subcomponents.totalMass);
    }

    part def Engine :> Component {
        attribute thrust :> ISQ::force;
        attribute specificImpulse :> ISQ::time;
    }

    part def 'F-1' :> Engine {
        attribute :>> mass = 8400 [kg];
        attribute :>> thrust = 6770000 [N];
        attribute :>> specificImpulse = 263 [s];
    }

    part def 'J-2' :> Engine {
        attribute :>> mass = 1788 [kg];
        attribute :>> thrust = 1033000 [N];
        attribute :>> specificImpulse = 421 [s];
    }

    part def Stage :> Component {
        attribute propellantMass :> ISQ::mass;
        attribute :>> totalMass = mass + propellantMass + sum(subcomponents.totalMass);
        part engines : Engine[*];
        part :>> subcomponents = engines;
    }

    part def 'S-IC' :> Stage {
        attribute :>> mass = 130000 [kg];
        attribute :>> propellantMass = 2160000 [kg];
        part engine1 : 'F-1';
        part engine2 : 'F-1';
        part engine3 : 'F-1';
        part engine4 : 'F-1';
        part engine5 : 'F-1';
        part :>> engines = (engine1, engine2, engine3, engine4, engine5);
    }

    part def 'S-II' :> Stage {
        attribute :>> mass = 36000 [kg];
        attribute :>> propellantMass = 444000 [kg];
        part engine1 : 'J-2';
        part engine2 : 'J-2';
        part engine3 : 'J-2';
        part engine4 : 'J-2';
        part engine5 : 'J-2';
        part :>> engines = (engine1, engine2, engine3, engine4, engine5);
    }

    part def 'S-IVB' :> Stage {
        attribute :>> mass = 10000 [kg];
        attribute :>> propellantMass = 109000 [kg];
        part engine1 : 'J-2';
        part :>> engines = engine1;
    }

    part def LaunchVehicle :> Component {
        attribute :>> mass = 0 [kg];
        part stage1 : 'S-IC';
        part stage2 : 'S-II';
        part stage3 : 'S-IVB';
        part :>> subcomponents = (stage1, stage2, stage3);
    }

    part saturnV : LaunchVehicle;

    // The same stack with an instrument unit whose mass is declared but never
    // given: well-formed SysML, and a rollup the runtime cannot complete.
    part def InstrumentUnit :> Component {
        attribute height :> ISQ::length = 0.91 [m];
    }

    part def LaunchVehicleWithIU :> Component {
        attribute :>> mass = 0 [kg];
        part stage1 : 'S-IC';
        part stage2 : 'S-II';
        part stage3 : 'S-IVB';
        part instrumentUnit : InstrumentUnit;
        part :>> subcomponents = (stage1, stage2, stage3, instrumentUnit);
    }

    part saturnVWithIU : LaunchVehicleWithIU;
}
