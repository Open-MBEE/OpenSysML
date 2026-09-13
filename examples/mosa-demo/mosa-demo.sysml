/*
 * MOSA demo: a modular ground vehicle worked through the Modular Open Systems
 * Approach with the OpenSysML `MOSA` library. Each package is stamped with the
 * MOSA pillar it serves; each artefact carries the statutory keyword that types
 * it and adds it to the corresponding base usage.
 *
 *   ./bin/sysml examples/mosa-demo/mosa-demo.sysml -validate
 */
package 'Modular Ground Vehicle' {
	private import MOSA::*;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import DocumentQueries::*;

	@MOSAPackage { pillar = MOSAPillar::openStandards; }
	package 'Open Standards' {
		/* The program's standards profile: the standards its interfaces are verified against. */
		item standardsProfile {
			#consensusStandard item ethernet : ConsensusStandard {
				doc /* Wired Ethernet: the physical and link layers of the sensor and radio data links. */
				:>> organization = "IEEE";
				:>> identifier = "802.3";
				:>> version = "2022";
				:>> registry = "https://standards.ieee.org/ieee/802.3/10422/";
			}

			#consensusStandard item canBus : ConsensusStandard {
				doc /* Controller area network for the vehicle's power management bus. */
				:>> organization = "ISO";
				:>> identifier = "11898-1";
				:>> version = "2015";
			}

			#technicalStandard item vehiclePower : Standard {
				doc /* Military ground vehicle 28 V DC electrical power characteristics: government-controlled,
				 * so a Standard but not a ConsensusStandard. */
				:>> organization = "US Department of Defense";
				:>> identifier = "MIL-STD-1275";
				:>> version = "E";
			}

			#technicalStandard item vehicleArchitecture : Standard {
				doc /* Vehicle electronics integration architecture maintained by a government-led consortium. */
				:>> organization = "VICTORY Standards Support Office";
				:>> identifier = "VICTORY";
				:>> version = "1.9";
			}
		}
	}

	@MOSAPackage { pillar = MOSAPillar::keyInterfaces; }
	package 'Modular System Interfaces' {
		port def PowerPort;
		port def DataPort;
		port def MountPort;
		port def RfPort;

		#modularSystemInterface interface def 'Sensor Data Link' {
			doc /* Imagery and detections from the sensor pod to the mission computer over Ethernet. */
			attribute :>> characteristics = (InterfaceCharacteristicKind::data, InterfaceCharacteristicKind::networking);
			end sensor : DataPort;
			end computer : DataPort;
		}

		#modularSystemInterface interface def 'Radio Data Link' {
			doc /* Reports and commands between the mission computer and the radio. */
			attribute :>> characteristics = (InterfaceCharacteristicKind::data, InterfaceCharacteristicKind::networking);
			end computer : DataPort;
			end radio : DataPort;
		}

		#modularSystemInterface interface def 'Power Bus' {
			doc /* 28 V DC power from the power unit to the mission computer. */
			attribute :>> characteristics = (InterfaceCharacteristicKind::electrical);
			end supply : PowerPort;
			end load : PowerPort;
		}

		#modularSystemInterface interface def 'Payload Mount' {
			doc /* The mechanical mount of the sensor pod on the chassis. */
			attribute :>> characteristics = (InterfaceCharacteristicKind::mechanical);
			end chassis : MountPort;
			end payload : MountPort;
		}

		#modularSystemInterface interface def 'Autonomy Command Link' {
			doc /* Drive commands from the autonomy module to the mission computer. */
			attribute :>> characteristics = (InterfaceCharacteristicKind::software);
			end autonomy : DataPort;
			end computer : DataPort;
		}
	}

	@MOSAPackage { pillar = MOSAPillar::modularDesign; }
	package 'Modular Design' {
		private import 'Modular System Interfaces'::*;
		private import 'Open Standards'::*;

		#majorSystemComponent part def Chassis {
			port mount : MountPort;
			port antennaFeed : RfPort;
		}

		#majorSystemComponent part def 'Mission Computer' {
			port sensorIn : DataPort;
			port radioOut : DataPort;
			port autonomyIn : DataPort;
			port power : PowerPort;
		}

		#majorSystemComponent part def 'Sensor Pod' {
			port data : DataPort;
			port mount : MountPort;
		}

		#majorSystemComponent part def Radio {
			port data : DataPort;
			port antenna : RfPort;
		}

		#majorSystemComponent part def 'Power Unit' {
			port power : PowerPort;
		}

		#modularSystem part def 'Autonomy Module' {
			doc /* Software that plans and drives without the coincident execution of the sensor pod or
			 * radio; it can be separated from the vehicle and recombined with another. */
			port command : DataPort;
		}

		item def 'Vendor Perception Library';

		/* The vehicle is the major system platform: the highest-level structure, on which the major
		 * system components are mounted. The rights the program holds in each component's technical
		 * data are recorded on the component. */
		#majorSystemPlatform part vehicle {
			#majorSystemComponent part chassis : Chassis {
				@DataRights { kind = DataRightsKind::unlimited; }
			}
			#majorSystemComponent part missionComputer : 'Mission Computer' {
				@DataRights {
					kind = DataRightsKind::governmentPurpose;
					basis = "Developed with mixed funding under the development contract.";
				}
			}
			#majorSystemComponent part sensorPod : 'Sensor Pod' {
				@DataRights {
					kind = DataRightsKind::limited;
					asserter = "Sensor vendor";
					basis = "Optical bench developed exclusively at private expense.";
				}
			}
			#majorSystemComponent part radio : Radio {
				doc /* No data rights are recorded for the radio: `-validate` reports it. */
			}
			#majorSystemComponent part powerUnit : 'Power Unit' {
				@DataRights { kind = DataRightsKind::unlimited; }
			}
			#modularSystem part autonomy : 'Autonomy Module' {
				@DataRights { kind = DataRightsKind::governmentPurpose; }
				item perception : 'Vendor Perception Library' {
					doc /* A proprietary element whose rationale for staying so is not recorded: `-validate`
					 * reports it. */
					@Proprietary { owner = "Autonomy vendor"; }
				}
			}

			/* The key interface of the program: competed sensor pods plug in here. */
			#keyInterface interface sensorLink : 'Sensor Data Link'
				connect sensorPod.data to missionComputer.sensorIn {
				@InterfaceControl { authority = "Program office interface control working group"; document = "MGV-ICD-001"; }
			}

			#modularSystemInterface interface radioLink : 'Radio Data Link'
				connect missionComputer.radioOut to radio.data {
				@InterfaceControl { authority = "Program office interface control working group"; document = "MGV-ICD-001"; }
			}

			#modularSystemInterface interface powerBus : 'Power Bus'
				connect powerUnit.power to missionComputer.power {
				doc /* No interface control authority is recorded: `-validate` reports it. */
			}

			#modularSystemInterface interface payloadMount : 'Payload Mount'
				connect chassis.mount to sensorPod.mount {
				doc /* Vendor-controlled, with the rationale MOSA asks for. */
				@Proprietary {
					owner = "Sensor vendor";
					rationale = "The mount is qualified with the vendor's vibration isolation; a consensus standard mount would require requalification of the optical bench.";
				}
				@InterfaceControl { authority = "Sensor vendor, under the program's configuration control"; document = "MGV-ICD-002"; }
			}

			#modularSystemInterface interface autonomyLink : 'Autonomy Command Link'
				connect autonomy.command to missionComputer.autonomyIn {
				doc /* Conforms to no standard and is not marked proprietary: `-validate` reports it. */
				@InterfaceControl { authority = "Program office interface control working group"; }
			}

			/* A boundary between two major system components that is not designated a modular
			 * system interface: `-validate` reports it. */
			connect radio.antenna to chassis.antennaFeed;

			/* Which standards each interface is verified against. */
			#conformance connection {
				end #conformant ::> sensorLink;
				end #conformsTo ::> standardsProfile.ethernet;
			}

			#conformance connection {
				end #conformant ::> radioLink;
				end #conformsTo ::> standardsProfile.ethernet;
				end #conformsTo ::> standardsProfile.vehicleArchitecture;
			}

			#conformance connection {
				end #conformant ::> powerBus;
				end #conformsTo ::> standardsProfile.vehiclePower;
				end #conformsTo ::> standardsProfile.canBus;
			}
		}
	}

	@MOSAPackage { pillar = MOSAPillar::modularDesign; }
	package 'MOSA Requirements' {
		private import 'Modular System Interfaces'::*;

		#mosaObjective requirement def 'Sensor Competition Objective' {
			doc /* The program expects to compete the sensor pod at each technology refresh. */
			subject platform : MajorSystemPlatform;
			#moe attribute sensorRefreshInterval : DurationValue;
		}

		#mosaObjective requirement sensorCompetition : 'Sensor Competition Objective';

		#modularityRequirement requirement def 'Severable Sensor Pod' {
			doc /* The sensor pod shall be removable and replaceable without change to the mission
			 * computer or the chassis. */
			subject platform : MajorSystemPlatform;
			#mop attribute replacementTime : DurationValue;
		}

		#modularityRequirement requirement severableSensor : 'Severable Sensor Pod';

		#derivation connection {
			end #original ::> sensorCompetition;
			end #derive ::> severableSensor;
		}

		#interfaceRequirement requirement def 'Open Sensor Link' {
			doc /* The sensor data link shall conform to IEEE 802.3 and be published in the interface
			 * control document. */
			subject link : 'Sensor Data Link';
		}

		#interfaceRequirement requirement openSensorLink : 'Open Sensor Link';

		#derivation connection {
			end #original ::> severableSensor;
			end #derive ::> openSensorLink;
		}

		#interfaceRequirement requirement def 'Standard Power Bus' {
			doc /* Power shall be supplied to MIL-STD-1275 characteristics over a CAN-managed bus. */
			subject bus : 'Power Bus';
		}

		#interfaceRequirement requirement standardPower : 'Standard Power Bus';

		#interfaceRequirement requirement def 'Controlled Payload Mount' {
			doc /* The payload mount shall be held under the program's configuration control. */
			subject mount : 'Payload Mount';
		}

		#interfaceRequirement requirement controlledMount : 'Controlled Payload Mount';

		#interfaceRequirement requirement def 'Documented Autonomy Link' {
			doc /* The autonomy command link shall be documented in machine-readable form. */
			subject link : 'Autonomy Command Link';
		}

		#interfaceRequirement requirement documentedAutonomyLink : 'Documented Autonomy Link';
	}

	@MOSAPackage { pillar = MOSAPillar::conformance; }
	@MOSAConformance {
		criterion = MOSAConformanceCriterion::keyInterfaces;
		status = StatusKind::open;
		evidence = "Four of five modular system interfaces conform to a standard; the autonomy command link is documented but conforms to none.";
	}
	package 'Conformance Assessment' {
		private import 'Modular Design'::vehicle;
		private import 'MOSA Requirements'::*;

		metadata MOSAConformance about vehicle {
			criterion = MOSAConformanceCriterion::modularity;
			status = StatusKind::done;
			evidence = "The sensor pod and autonomy module are severable across designated interfaces.";
		}

		satisfy openSensorLink by vehicle.sensorLink;
		satisfy standardPower by vehicle.powerBus;
		satisfy controlledMount by vehicle.payloadMount;
		satisfy documentedAutonomyLink by vehicle.autonomyLink;
		/* The radio data link is the subject of no interface requirement: `-validate` reports it. */
	}

	/* The approach's standard products over this model: `-render-all` draws each of them. */
	@MOSAPackage { pillar = MOSAPillar::enablingEnvironment; }
	package 'MOSA Views' {
		view decomposition : ModularDecompositionView { expose 'Modular Design'::**; }
		view interfaces : ModularInterfaceView { expose 'Modular Design'::vehicle; }
		view interfaceRegister : InterfaceRegisterView { expose 'Modular Design'::vehicle::*; }
		view standards : StandardsView { expose 'Open Standards'::standardsProfile::*; }
		view proprietary : ProprietaryElementsView { expose 'Modular Design'::**; }
		view dataRights : DataRightsView { expose 'Modular Design'::vehicle::*; }
		view requirements : MOSARequirementsView { expose 'MOSA Requirements'::*; }
		view conformance : ConformanceView {
			expose 'Conformance Assessment';
			expose 'Modular Design'::vehicle;
		}
	}

	/* The interface control document, generated from the model: `-render-document` writes it. */
	@MOSAPackage { pillar = MOSAPillar::enablingEnvironment; }
	package 'Interface Control Document' {
		private import 'Modular Design'::vehicle;
		private import 'Open Standards'::standardsProfile;

		part def 'Ground Vehicle Interface Control Document' :> InterfaceControlDocument {
			:>> title = "Modular Ground Vehicle Interface Control Document";

			part scope : Paragraph {
				:>> text = "This document publishes the modular system interfaces of the Modular Ground Vehicle, the standards each is verified against, and the rights the program holds in the technical data of the components they connect. It is generated from the vehicle model.";
			}

			part interfaces : Section {
				:>> title = "Modular system interfaces";
				part register : Table {
					:>> caption = "Interfaces of the vehicle, with their characteristics";
					calc rows : InterfaceRegister { in :>> root = vehicle; }
				}
			}

			part standards : Section {
				:>> title = "Standards";
				part register : Table {
					:>> caption = "Standards the interfaces are verified against";
					calc rows : StandardsRegister { in :>> root = standardsProfile; }
				}
			}

			part rights : Section {
				:>> title = "Technical data rights";
				part register : Table {
					:>> caption = "Components whose data rights are recorded";
					calc rows : DataRightsRegister { in :>> root = vehicle; }
				}
				part proprietary : Table {
					:>> caption = "Proprietary elements";
					calc rows : ProprietaryRegister { in :>> root = vehicle; }
				}
			}
		}
	}
}
