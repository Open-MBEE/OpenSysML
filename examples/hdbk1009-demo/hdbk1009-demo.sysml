/*
 * Dust Lidar Pathfinder: a small fictional science project modelled with the HDBK1009 library,
 * following the metamodel of NASA-HDBK-1009A from the stakeholders down to the verification and
 * validation requirements, with the tables of sections 8.1 to 8.22 generated from the model. The
 * gaps the HDBK1009 checks report are left open on purpose and marked with a doc comment.
 */
package 'Dust Lidar Pathfinder Model' {
	private import HDBK1009::*;
	private import ScalarValues::*;
	private import DocumentQueries::*;

	part pathfinder {
		package Stakeholders {
			#stakeholderActor part scienceTeam { doc /* The lunar dust science team, who use the data. */ }
			#stakeholderActor part landerProvider { doc /* The provider of the host lander. */ }
		}

		package Expectations {
			private import Stakeholders::*;

			#stakeholderExpectationStatement requirement <'ses-1'> dustEnvironmentKnown {
				doc /* The science team expects to know how much dust the lander's plume lofts. */
				:>> id = "ses-1";
			}
			dependency from dustEnvironmentKnown to scienceTeam;

			#stakeholderExpectationStatement requirement <'ses-2'> noHarmToLander {
				doc /* The lander provider expects the instrument to put no load on the lander beyond its allocation. */
				:>> id = "ses-2";
			}
			dependency from noHarmToLander to landerProvider;

			#need requirement <'n-1'> characterizeDust {
				doc /* Characterize the dust lofted during and after landing. */
				:>> id = "n-1";
			}
			dependency from characterizeDust to dustEnvironmentKnown;

			#goal requirement <'g-1'> measureDustProfile {
				doc /* Measure the vertical profile of lofted dust above the landing site. */
				:>> id = "g-1";
			}
			dependency from measureDustProfile to dustEnvironmentKnown;
			#derivation connection { end #original ::> characterizeDust; end #derive ::> measureDustProfile; }

			#missionObjective requirement <'o-1'> profileToOneKilometre {
				doc /* Profile dust density from the surface to 1 km altitude for one lunar day. */
				:>> id = "o-1";
			}
			dependency from profileToOneKilometre to dustEnvironmentKnown;
			#derivation connection { end #original ::> measureDustProfile; end #derive ::> profileToOneKilometre; }

			#missionObjective requirement <'o-2'> stayWithinAllocation {
				doc /* Operate within the mass and power the lander allocates. This objective is traced
				 * to no expectation statement, derives from no goal and has no validation
				 * requirement. */
				:>> id = "o-2";
			}
		}

		package Measures {
			private import Expectations::*;
			private import Requirements::*;

			#measureOfEffectiveness requirement <'moe-1'> profilesReturned {
				doc /* The fraction of planned dust profiles returned to the science team. */
				:>> id = "moe-1";
			}
			#refinement dependency from profilesReturned to profileToOneKilometre;

			#measureOfPerformance requirement <'mop-1'> detectionRange {
				doc /* The altitude to which the lidar detects dust at the reference density. */
				:>> id = "mop-1";
			}
			#derivation connection { end #original ::> profilesReturned; end #derive ::> detectionRange; }
			#refinement dependency from detectionRange to rangeRequirement;

			#measureOfPerformance requirement <'mop-2'> instrumentMass {
				doc /* The mass of the instrument. This measure derives from no measure of
				 * effectiveness. */
				:>> id = "mop-2";
			}
			#refinement dependency from instrumentMass to massRequirement;
		}

		package Requirements {
			private import Expectations::*;

			#systemRequirement requirement <'sys-1'> rangeRequirement {
				doc /* The lidar shall detect dust at the reference density to an altitude of 1 km. */
				:>> id = "sys-1";
				:>> verifyMethod = VerificationMethodKind::test;
			}
			#derivation connection { end #original ::> profileToOneKilometre; end #derive ::> rangeRequirement; }

			#systemRequirement requirement <'sys-2'> massRequirement {
				doc /* The instrument shall have a mass of no more than 12 kg. */
				:>> id = "sys-2";
				:>> verifyMethod = VerificationMethodKind::analyze;
			}
			#derivation connection { end #original ::> stayWithinAllocation; end #derive ::> massRequirement; }

			#subsystemRequirement requirement <'sub-1'> transmitterEnergy {
				doc /* The transmitter shall emit pulses of at least 2 mJ. */
				:>> id = "sub-1";
			}
			#derivation connection { end #original ::> rangeRequirement; end #derive ::> transmitterEnergy; }

			#componentRequirement requirement <'cmp-1'> laserDiodeCurrent {
				doc /* The laser diode driver shall supply 40 A pulses. No requirement is its origin and
				 * no V&V case verifies it. */
				:>> id = "cmp-1";
			}
		}

		package Structure {
			private import Requirements::*;
			private import Measures::*;

			#systemContext part landingSite {
				#externalElement part lander;
				#systemUser part operator;
				#missionElement part lidar {
					#tpm attribute detectionRangeKm : Real = 1.2;
					#tpm attribute massKg : Real = 11.4;
					#systemElement part transmitter {
						#componentElement part laserDiodeDriver;
					}
					#systemElement part receiver;
				}
			}
			satisfy rangeRequirement by landingSite.lidar;
			satisfy massRequirement by landingSite.lidar;
			satisfy transmitterEnergy by landingSite.lidar.transmitter;
			satisfy detectionRange by landingSite.lidar.detectionRangeKm;
			satisfy instrumentMass by landingSite.lidar.massKg;
			satisfy profilesReturned by landingSite.lidar;
		}

		package Behavior {
			private import Expectations::*;
			private import Structure::*;

			#systemUseCase use case profileDust;
			#refinement dependency from profileDust to profileToOneKilometre;

			#missionActivity action surfaceOperations {
				#systemFunction action emitPulse {
					#componentFunction action driveDiode;
				}
				#systemFunction action collectReturn;
			}
			#refinement dependency from surfaceOperations to profileToOneKilometre;
			allocate surfaceOperations to landingSite.lidar;
			allocate surfaceOperations.emitPulse to landingSite.lidar.transmitter;
			allocate surfaceOperations.collectReturn to landingSite.lidar.receiver;
		}

		package 'Verification and Validation' {
			private import Expectations::*;
			private import Requirements::*;
			private import Structure::*;

			#verificationRequirement requirement <'ver-1'> rangeTest {
				doc /* Fire the lidar down a 1 km horizontal range through the reference aerosol and record the return. */
				:>> id = "ver-1";
				:>> verifyMethod = VerificationMethodKind::test;
				:>> successCriteria = "The return at 1 km exceeds the detection threshold in 95 of 100 shots.";
				:>> closureStatus = "Open";
			}

			#verificationRequirement requirement <'ver-2'> massAnalysis {
				doc /* Sum the measured masses of the flight components. No V&V case refines this
				 * requirement and it states no method. */
				:>> id = "ver-2";
				:>> successCriteria = "The sum is no more than 12 kg.";
			}

			#validationRequirement requirement <'val-1'> fieldCampaign {
				doc /* Operate the engineering model for one diurnal cycle at the analogue site with the science team. */
				:>> id = "val-1";
				:>> verifyMethod = VerificationMethodKind::demo;
				:>> successCriteria = "The science team accepts the returned profiles.";
			}
			satisfy profileToOneKilometre by fieldCampaign;

			#vvConfiguration part rangeTestSetup {
				ref part :>> articlesUnderTest = landingSite.lidar;
				part :>> personnel = testConductor;
				part :>> equipment = aerosolChamber;
				part :>> facilities = opticalRange;
				#vvPersonnel part testConductor;
				#vvEquipment part aerosolChamber;
				#vvFacility part opticalRange;
			}

			#vvEvent action instrumentTestCampaign;
			#vvArtifact item rangeTestReport;
			satisfy rangeTest by rangeTestReport;

			#vvCase verification rangeTestCase {
				objective {
					verify rangeRequirement;
					verify transmitterEnergy;
				}
				ref part :>> configurations = rangeTestSetup;
				#vvCaseActivity action runRangeTest {
					#vvStep action alignLidar;
					#vvStep action fireAndRecord;
				}
			}
			#refinement dependency from rangeTestCase to rangeTest;
			allocate rangeTest to instrumentTestCampaign;

			#vvCase verification massRollUp {
				objective { verify massRequirement; }
			}

			#vvCase verification fieldCampaignCase;
			#refinement dependency from fieldCampaignCase to fieldCampaign;
			allocate fieldCampaign to instrumentTestCampaign;
		}
	}

	package Products {
		part def 'Pathfinder Stakeholder Expectations' :> StakeholderExpectations {
			:>> title = "Dust Lidar Pathfinder Stakeholder Expectations";
			part stakeholders : Section {
				:>> title = "Stakeholders";
				part table : Table {
					:>> caption = "Stakeholders";
					calc rows : StakeholderRegister { in :>> root = pathfinder; }
				}
			}
			part expectations : Section {
				:>> title = "Expectations, needs, goals and objectives";
				part table : Table {
					:>> caption = "Expectation statements, needs, goals and objectives with their traceability";
					calc rows : ExpectationRegister { in :>> root = pathfinder; }
				}
			}
		}
		part stakeholderExpectations : 'Pathfinder Stakeholder Expectations';

		part def 'Pathfinder Technical Requirements' :> TechnicalRequirementsDocument {
			:>> title = "Dust Lidar Pathfinder Technical Requirements";
			part requirements : Section {
				:>> title = "Requirements";
				part table : Table {
					:>> caption = "Technical requirements with their traceability";
					calc rows : RequirementsRegister { in :>> root = pathfinder; }
				}
			}
			part measures : Section {
				:>> title = "Measures";
				part table : Table {
					:>> caption = "Measures of effectiveness and of performance";
					calc rows : MeasureRegister { in :>> root = pathfinder; }
				}
			}
		}
		part technicalRequirements : 'Pathfinder Technical Requirements';

		part def 'Pathfinder Verification Matrices' :> VerificationRequirementsMatrix {
			:>> title = "Dust Lidar Pathfinder Verification Matrices";
			part byRequirement : Section {
				:>> title = "Requirements verification matrix";
				part table : Table {
					:>> caption = "Requirements and the V&V cases verifying them";
					calc rows : RequirementsVerificationRegister { in :>> root = pathfinder; }
				}
			}
			part byVerificationRequirement : Section {
				:>> title = "Verification requirements matrix";
				part table : Table {
					:>> caption = "Verification requirements";
					calc rows : VerificationRequirementsRegister { in :>> root = pathfinder; }
				}
			}
		}
		part verificationMatrices : 'Pathfinder Verification Matrices';

		part def 'Pathfinder Validation Matrix' :> ValidationRequirementsMatrix {
			:>> title = "Dust Lidar Pathfinder Validation Requirements Matrix";
			part matrix : Section {
				:>> title = "Validation requirements matrix";
				part table : Table {
					:>> caption = "Validation requirements";
					calc rows : ValidationRequirementsRegister { in :>> root = pathfinder; }
				}
			}
		}
		part validationMatrix : 'Pathfinder Validation Matrix';
	}
}
