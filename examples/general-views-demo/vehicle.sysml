// A vehicle, its requirements and the cases verifying them, drawn by four
// GeneralViews: a requirement graph, a definition and usage graph, a package
// graph, and an unfiltered view that stays a containment tree. See README.md.
package VehicleStructure {
	private import ScalarValues::*;

	part def Vehicle {
		attribute mass : Real;
		part chassis : Chassis;
		ref part driver : Driver;
	}
	part def Chassis;
	part def LightChassis :> Chassis;
	part def Driver;
	action def Brake;

	part vehicle : Vehicle {
		attribute :>> mass = 1500.0;
	}
	part heavyVehicle : Vehicle {
		attribute :>> mass = 2500.0;
	}
	action brake : Brake;

	package Powertrain {
		part def Engine;
	}
}

package VehicleRequirements {
	private import RequirementDerivation::*;
	private import ModelingMetadata::*;
	private import ScalarValues::*;
	private import VehicleStructure::*;

	requirement def <'R1'> MassRequirement {
		doc /* The total mass of the vehicle shall not exceed the stated mass limit, measured empty and without a driver. */
		subject vehicle : Vehicle;
		attribute massLimit : Real;
		require constraint { vehicle.mass <= massLimit }
	}
	requirement def <'R2'> StoppingRequirement {
		doc /* The vehicle shall stop within the stated distance. */
	}
	requirement def <'R3'> EmergencyStopRequirement :> StoppingRequirement;

	requirement <'R1.1'> vehicleMass : MassRequirement {
		attribute :>> massLimit = 2000.0;
	}
	requirement <'R1.2'> chassisMass : MassRequirement;
	requirement <'R3.1'> emergencyStop : EmergencyStopRequirement;

	#derivation connection {
		end #original ::> vehicleMass;
		end #derive ::> chassisMass;
	}
	#refinement dependency LightChassis to chassisMass;

	satisfy vehicleMass by vehicle;
	satisfy vehicleMass by heavyVehicle;
	satisfy emergencyStop by vehicle;
	allocate emergencyStop to brake;
}

package VehicleVerification {
	private import VehicleStructure::*;
	private import VehicleStructure::Powertrain::*;
	private import VehicleRequirements::*;

	verification def MassTest {
		subject testVehicle : Vehicle;
		objective {
			verify vehicleMass;
		}
		VerificationCases::PassIf(testVehicle.mass <= 2000.0)
	}
	verification lightMassTest : MassTest {
		subject testVehicle = vehicle;
	}
	verification heavyMassTest : MassTest {
		subject testVehicle = heavyVehicle;
	}
	verification def StopTest {
		subject testVehicle : Vehicle;
		objective {
			verify emergencyStop;
		}
	}
	verification stopTest : StopTest {
		subject testVehicle = vehicle;
	}
}

package GeneralViews {
	private import StandardViewDefinitions::*;

	view requirementView : GeneralView {
		filter @SysML::RequirementDefinition or @SysML::RequirementUsage;
		expose VehicleRequirements::**;
	}
	view definitionView : GeneralView {
		expose VehicleStructure::**[@SysML::Definition or @SysML::Usage];
	}
	view packageView : GeneralView {
		filter @SysML::Package;
		expose VehicleStructure;
		expose VehicleStructure::Powertrain;
		expose VehicleRequirements;
		expose VehicleVerification;
		expose GeneralViews;
	}
	view plainView : GeneralView {
		expose VehicleStructure::*;
	}
}
