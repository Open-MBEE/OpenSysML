doc /* Author: demo team
 * Created: 2026-09-05
 */
package 'Vehicle Design' {
    doc /* Structural model of the demo vehicle. */
    package 'Value Types' {
        attribute def Mass :> ScalarValues::Real {
            /* «ValueType» tags with no v2 form: quantityKind = Vehicle Design::Value Types::mass; unit = Vehicle Design::Value Types::kilogram */
        }
        attribute def Speed :> Mass;
        /* not migrated: «Unit» InstanceSpecification 'kilogram' — units and quantity kinds are not migrated; use the SI and ISQ libraries; applied stereotypes «Unit» (quantityKind = Vehicle Design::Value Types::mass; symbol = kg) */
        /* not migrated: «QuantityKind» InstanceSpecification 'mass' — units and quantity kinds are not migrated; use the SI and ISQ libraries; applied stereotypes «QuantityKind» */
        enum def Color {
            red;
            green;
            blue;
        }
        attribute def StartSignal {
            attribute key : ScalarValues::Integer;
        }
    }
    package Interfaces {
        port def FuelInterface {
            in item fuel : Fuel;
            out attribute rate : 'Vehicle Design'::'Value Types'::Speed;
        }
        part def Fuel;
    }
    part def Vehicle :> System {
        doc /* A road vehicle. Mass is limited by the mass requirement. */
        comment /* Second note on the vehicle. */
        attribute mass : 'Vehicle Design'::'Value Types'::Mass default = 1200.0;
        attribute color : 'Vehicle Design'::'Value Types'::Color default = 'Vehicle Design'::'Value Types'::Color::red;
        attribute 'vehicle name' : ScalarValues::String default = "the \"demo\" car";
        constant attribute isElectric : ScalarValues::Boolean default = false;
        derived attribute totalMass : 'Vehicle Design'::'Value Types'::Mass default = mass + engine.mass;
        part engine : Engine[1..2];
        part wheels : Wheel[4..*];
        ref part driver : Driver[0..1];
        private ref part sharedCharger : Engine;
        constraint massLimit : MassLimit;
        port fuelIn : ~'Vehicle Design'::Interfaces::FuelInterface;
        out port speedOut {
            out attribute speedOut : 'Vehicle Design'::'Value Types'::Speed;
        }
        port diag;
        connection 'fuel line' connect fuelIn to engine.fuelPort;
        flow fuelIn.fuel to engine.fuelPort.fuel;
        bind mass = massLimit.m;
        connect speedOut to engine.piston.p;
        /* not migrated: Operation 'start' — operations and receptions are not migrated; v2 has no operation */
        /* not migrated: StateMachine 'Vehicle States' — behaviors are not migrated yet */
        constraint 'positive mass' { mass > 0 }
        part def Chassis {
            attribute mass : 'Vehicle Design'::'Value Types'::Mass;
        }
        satisfy requirement : Requirements::'Mass Requirement';
        satisfy requirement : Requirements::'Engine Mass Requirement' by engine;
        /* «Block» tags with no v2 form: isEncapsulated = true */
    }
    abstract part def System;
    part def Engine {
        attribute mass : 'Vehicle Design'::'Value Types'::Mass;
        part piston : Piston;
        port fuelPort : 'Vehicle Design'::Interfaces::FuelInterface;
        in port speedIn {
            in attribute speedIn : 'Vehicle Design'::'Value Types'::Speed;
        }
        /* applied stereotype «Critical»: level = high */
    }
    part def Piston {
        in port p {
            in attribute p : 'Vehicle Design'::'Value Types'::Speed;
        }
    }
    part def Wheel;
    part def 'Electric Vehicle' :> Vehicle {
        part engine : Motor :>> engine;
        part spare : Wheel :> wheels;
    }
    part def Motor :> Engine;
    part def Driver;
    part def Helper;
    connection def Drives {
        end driver : Driver[0..1];
        end vehicle : Vehicle;
    }
    constraint def MassLimit {
        in attribute m : 'Vehicle Design'::'Value Types'::Mass;
        in attribute limit : 'Vehicle Design'::'Value Types'::Mass;
        m < limit
    }
    individual def myCar :> Vehicle {
        attribute :>> mass = 1350.5;
        attribute :>> color = 'Vehicle Design'::'Value Types'::Color::blue;
    }
    /* not migrated: Activity 'Drive' — behaviors are not migrated yet */
}
package Requirements {
    requirement def <R1> 'Mass Requirement' {
        doc /* The vehicle shall have a mass of less than 1500 kg. */
        requirement def <'R1.1'> 'Chassis Mass' {
            doc /* The chassis shall have a mass of less than 400 kg. */
        }
    }
    requirement def <R2> 'Speed Requirement' {
        doc /* The vehicle shall reach 100 km/h "quickly". */
    }
    requirement def <'R1.2'> 'Engine Mass Requirement' {
        doc /* The engine shall have a mass of less than 300 kg. */
    }
    connection def 'Derive Engine Mass Requirement' :> RequirementDerivation::Derivation {
        end #RequirementDerivation::original originalRequirement : 'Mass Requirement';
        end #RequirementDerivation::derive derivedRequirement : 'Engine Mass Requirement';
    }
    /* not migrated: «Verify» Abstraction (_dep_verify_block) — a verify whose client is not a test case has no v2 form; applied stereotypes «Verify» */
    /* not migrated: «Refine» Abstraction (_dep_refine) — its end Vehicle Design::Drive is not migrated; applied stereotypes «Refine» */
    dependency 'Speed Requirement' to 'Vehicle Design'::Engine; /* «Trace» */
    /* not migrated: «Allocate» Abstraction (_dep_allocate) — its end Vehicle Design::Drive is not migrated; applied stereotypes «Allocate» */
    allocate 'Vehicle Design'::Motor to 'Vehicle Design'::Engine;
    dependency 'Vehicle Design'::Wheel to 'Vehicle Design'::Engine;
    verification def 'Mass Test' {
        objective {
            verify requirement : 'Mass Requirement';
        }
    }
}
package 'Empty Package';
