/*
 * Regolith Sample Imager: a small fictional lunar-surface instrument modelled with the NPR7150
 * library. Its systems containing software are classified by Appendix D of NPR 7150.2D, its
 * Class C flight software has a Requirements Mapping Matrix generated from the model, and the
 * software assurance tasks of NASA-STD-8739.8B are assigned for one requirement. The gaps the
 * NPR7150 checks report are left open on purpose and marked with a doc comment.
 */
package 'Regolith Sample Imager' {
	private import NPR7150::*;
	private import NPR7150::SWERequirements::*;
	private import ScalarValues::*;
	private import DocumentQueries::*;

	package 'Software Classification' {
		doc /* Each system and subsystem containing software with its software class (SWE-020) and
		 * its safety-critical determination (SWE-205). */

		part def 'Analysis Tool' :> SoftwareSystem {
			doc /* Ground tools for secondary science data analysis are Class D. */
			:>> softwareClass = SoftwareClass::D;
		}

		part softwareInventory {
			#softwareSystem part imagerFlightSoftware {
				doc /* Commands the imager and its sample-illumination laser: Class C, software for a
				 * NASA Class D payload, and safety-critical because it controls a laser hazard. */
				:>> softwareClass = SoftwareClass::C;
				:>> safetyCritical = true;
			}

			#softwareSystem part groundDataPipeline {
				doc /* Processes the imager's science data on the ground: Class C. */
				:>> softwareClass = SoftwareClass::C;
				:>> safetyCritical = false;
			}

			part grainSizeEstimator : 'Analysis Tool';

			#softwareSystem part calibrationScripts {
				doc /* Deliberate gap: no software class is stated. */
			}

			#softwareSystem part laserInterlockPrototype {
				doc /* Deliberate gap: Class E software cannot be safety-critical. */
				:>> softwareClass = SoftwareClass::E;
				:>> safetyCritical = true;
			}
		
		}
	}

	package 'Mapping Entries' {
		doc /* The project's mapping to each requirement Table 2 of Appendix C marks for Class C
		 * software. The Requirements Mapping Matrix generates its table from these entries.
		 * SWE-058 is left out on purpose. */

		part mappingEntries {
			requirement swe033 : SWE033 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe013 : SWE013 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe024 : SWE024 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe034 : SWE034 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe036 : SWE036 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe037 : SWE037 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe039 : SWE039 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe040 : SWE040 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe042 : SWE042 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe139 : SWE139 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe121 : SWE121 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe125 : SWE125 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe027 : SWE027 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe015 : SWE015 { @RequirementMapping { comply = MappingKind::tailored; justification = "One cost estimate model is kept; the software is Class C and item c applies."; approvedBy = "Center Engineering Technical Authority"; } }
			requirement swe151 : SWE151 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe174 : SWE174 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe016 : SWE016 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe018 : SWE018 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe046 : SWE046 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe017 : SWE017 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe020 : SWE020 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe176 : SWE176 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe022 : SWE022 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe205 : SWE205 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe023 : SWE023 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe134 : SWE134 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			/* Deliberate gap: a safety-critical software requirement mapped not applicable. */
			requirement swe219 : SWE219 { @RequirementMapping { comply = MappingKind::notApplicable; justification = "The flight software reuses a heritage executive tested to statement coverage."; } }
			/* Deliberate gap: tailored with no justification. */
			requirement swe220 : SWE220 { @RequirementMapping { comply = MappingKind::tailored; } }
			requirement swe146 : SWE146 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe206 : SWE206 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe147 : SWE147 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe148 : SWE148 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe156 : SWE156 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe154 : SWE154 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe157 : SWE157 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe159 : SWE159 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe207 : SWE207 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe185 : SWE185 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe210 : SWE210 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe052 : SWE052 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe050 : SWE050 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe051 : SWE051 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe184 : SWE184 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe053 : SWE053 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe054 : SWE054 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe055 : SWE055 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe057 : SWE057 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe143 : SWE143 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe060 : SWE060 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe061 : SWE061 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe135 : SWE135 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe062 : SWE062 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe186 : SWE186 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe063 : SWE063 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe136 : SWE136 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe065 : SWE065 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe066 : SWE066 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe187 : SWE187 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe068 : SWE068 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe070 : SWE070 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe071 : SWE071 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe073 : SWE073 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe189 : SWE189 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe190 : SWE190 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe191 : SWE191 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe192 : SWE192 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe211 : SWE211 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe075 : SWE075 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe077 : SWE077 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe194 : SWE194 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe195 : SWE195 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe196 : SWE196 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe079 : SWE079 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe080 : SWE080 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe081 : SWE081 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe082 : SWE082 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe083 : SWE083 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe084 : SWE084 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe085 : SWE085 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe045 : SWE045 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe086 : SWE086 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe087 : SWE087 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe088 : SWE088 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe089 : SWE089 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe090 : SWE090 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe093 : SWE093 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe094 : SWE094 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe199 : SWE199 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe201 : SWE201 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe202 : SWE202 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement swe203 : SWE203 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
		}
	}

	package 'Software Assurance' {
		doc /* The NASA-STD-8739.8B Table 1 tasks software assurance performs for SWE-134, the
		 * safety-critical software requirement the flight software implements. */
		private import NPR7150::SoftwareAssurance::*;

		part assuranceTasks {
			requirement analyzeDesign : SA134_1 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement assessCode : SA134_2 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement confirmLoadedData : SA134_3 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement analyzeIsolation : SA134_4 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement participateInReviews : SA134_5 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
			requirement checkHazardAnalysis : SA134_6 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
		}
	}

	package 'Requirements Mapping Matrix' {
		private import 'Mapping Entries'::mappingEntries;
		private import 'Software Classification'::softwareInventory;
		private import 'Software Assurance'::assuranceTasks;

		part def 'Imager Flight Software Mapping Matrix' :> RequirementsMappingMatrix {
			:>> title = "Regolith Sample Imager Flight Software Requirements Mapping Matrix";
			:>> softwareClass = SoftwareClass::C;

			part scope : Paragraph {
				:>> text = "The Requirements Mapping Matrix of NPR 7150.2D Appendix C for the Class C imager flight software, generated from the mapping entries of the project model.";
			}

			part entries : Section {
				:>> title = "Requirements mapping matrix";
				part matrix : Table {
					:>> caption = "Mapping to the requirements invoked for Class C software";
					calc rows : RequirementsMappingRegister { in :>> root = mappingEntries; }
				}
			}

			part assurance : Section {
				:>> title = "Software assurance tasks";
				part tasks : Table {
					:>> caption = "NASA-STD-8739.8B tasks mapped for SWE-134";
					calc rows : AssuranceTaskRegister { in :>> root = assuranceTasks; }
				}
			}

			part classification : Section {
				:>> title = "Software classification";
				part systems : Table {
					:>> caption = "Systems and subsystems containing software";
					calc rows : SoftwareClassificationRegister { in :>> root = softwareInventory; }
				}
			}
		}

		part mappingMatrix : 'Imager Flight Software Mapping Matrix';
	}
}
