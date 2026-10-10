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
        item def StartSignal {
            attribute key : ScalarValues::Integer;
        }
    }
    package Interfaces {
        port def FuelInterface {
            in item fuel : Fuel[1];
            out attribute rate : 'Vehicle Design'::'Value Types'::Speed[1];
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
        ref part driver :> Driver[0..1];
        private ref part sharedCharger : Engine;
        constraint massLimit : MassLimit;
        port fuelIn : ~'Vehicle Design'::Interfaces::FuelInterface;
        out port speedOut[1] {
            out attribute speedOut : 'Vehicle Design'::'Value Types'::Speed;
        }
        port diag;
        binding 'fuel line' bind fuelIn = engine.fuelPort;
        flow of 'Vehicle Design'::Interfaces::Fuel from fuelIn.fuel to engine.fuelPort.fuel;
        bind mass = massLimit.m;
        bind speedOut = engine.piston.p;
        abstract action def start {
            in key : ScalarValues::Integer[1];
        }
        action 'start 2' : start;
        state def 'Vehicle States' {
            /* the region has no initial pseudostate: nothing enters it */
            state Off;
        }
        assert constraint 'positive mass' { mass > 0 }
        part def Chassis {
            attribute mass : 'Vehicle Design'::'Value Types'::Mass;
        }
        satisfy RequirementsModel::'Mass Requirement';
        satisfy RequirementsModel::'Engine Mass Requirement' by engine;
        /* «Block» tags with no v2 form: isEncapsulated = true */
    }
    abstract part def System;
    part def Engine {
        attribute mass : 'Vehicle Design'::'Value Types'::Mass;
        part piston : Piston;
        port fuelPort : 'Vehicle Design'::Interfaces::FuelInterface;
        in port speedIn[1] {
            in attribute speedIn : 'Vehicle Design'::'Value Types'::Speed;
        }
        /* applied stereotype «Critical»: level = high */
    }
    part def Piston {
        in port p[1] {
            in attribute p : 'Vehicle Design'::'Value Types'::Speed;
        }
    }
    part def Wheel;
    part def 'Electric Vehicle' :> Vehicle {
        part engine : Motor :>> engine;
        part spare : Wheel :> wheels;
    }
    part def Motor :> Engine;
    part Driver;
    occurrence def Helper;
    connection def Drives {
        end driver :> Driver[0..1];
        end vehicle : Vehicle;
    }
    constraint def MassLimit {
        in attribute m : 'Vehicle Design'::'Value Types'::Mass[1];
        in attribute limit : 'Vehicle Design'::'Value Types'::Mass[1];
        m < limit
    }
    individual part def myCar :> Vehicle {
        attribute :>> mass = 1350.5;
        attribute :>> color = 'Vehicle Design'::'Value Types'::Color::blue;
    }
    action def Drive;
    part vehicle : Vehicle;
    connection drives : Drives connect Driver to vehicle;
    action drive : Drive;
}
package RequirementsModel {
    requirement <R1> 'Mass Requirement' {
        doc /* The vehicle shall have a mass of less than 1500 kg. */
        requirement <'R1.1'> 'Chassis Mass' {
            doc /* The chassis shall have a mass of less than 400 kg. */
        }
    }
    requirement <R2> 'Speed Requirement' {
        doc /* The vehicle shall reach 100 km/h "quickly". */
    }
    requirement <'R1.2'> 'Engine Mass Requirement' {
        doc /* The engine shall have a mass of less than 300 kg. */
    }
    connection def 'Derive Engine Mass Requirement' :> RequirementDerivation::Derivation {
        end #RequirementDerivation::original originalRequirement :> 'Mass Requirement';
        end #RequirementDerivation::derive derivedRequirement :> 'Engine Mass Requirement';
    }
    /* not migrated: «Verify» Abstraction (_dep_verify_block) — a verify whose client is not a test case has no v2 form; applied stereotypes «Verify» */
    dependency 'Speed Requirement' to 'Vehicle Design'::Engine; /* «Trace» */
    allocation def 'Drive to Engine' {
        end :>> source : 'Vehicle Design'::Drive;
        end :>> target : 'Vehicle Design'::Engine;
    }
    allocation def 'Motor to Engine' {
        end :>> source : 'Vehicle Design'::Motor;
        end :>> target : 'Vehicle Design'::Engine;
    }
    dependency 'Vehicle Design'::Wheel to 'Vehicle Design'::Engine;
    verification def 'Mass Test' {
        objective {
            verify 'Mass Requirement';
        }
    }
    metadata MigrationMetadata::SynthesizedName about 'Drive to Engine', 'Motor to Engine';
}
package 'Empty Package';
metadata MigrationMetadata::LibraryNameAvoided about RequirementsModel { sourceName = "Requirements"; }
connection 'Drive refines Speed Requirement' connect 'Vehicle Design'::drive to RequirementsModel::'Speed Requirement';
view 'Vehicle BDD' {
    render Views::asTextualNotation;
}
