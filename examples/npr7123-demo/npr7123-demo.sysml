/*
 * Dust Lidar Pathfinder: a small fictional science project modelled with the NPR7123 library.
 * Its SEMP tailors the 17 common technical processes, its SRR and PDR state tailored Appendix G
 * criteria and the section 5.2.2.2 minimum products at their maturity, a technology carries a TRL,
 * and a compliance matrix generates Table H-1 from the model. The gaps the NPR7123 checks report
 * are left open on purpose and marked with a doc comment.
 */
package 'Dust Lidar Pathfinder' {
	private import NPR7123::*;
	private import NPR7123::SERequirements::*;
	private import ScalarValues::*;
	private import DocumentQueries::*;

	package 'Technical Planning' {
		doc /* The SEMP states how each of the 17 common technical processes is applied, as tailored
		 * (SE-58), and the measures the technical team tracks (SE-60 to SE-63). */

		#semp part pathfinderSEMP : SEMP {
			:>> title = "Dust Lidar Pathfinder Systems Engineering Management Plan";
			:>> maturity = WorkProductMaturity::baseline;

			#stakeholderExpectationsDefinition action defineStakeholderExpectations {
				:>> tailoring = "Customized: expectations are elicited in two science-team workshops and recorded in the model.";
			}
			#technicalRequirementsDefinition action defineRequirements {
				:>> tailoring = "Applied as written; requirements are held in the model and baselined at SRR.";
			}
			#logicalDecomposition action decomposeLogically {
				:>> tailoring = "Applied as written at the instrument and subsystem layers.";
			}
			#designSolutionDefinition action defineDesignSolution {
				:>> tailoring = "Applied as written; one design iteration between PDR and CDR.";
			}
			#productImplementation action implementProducts {
				:>> tailoring = "Customized: the lidar transmitter is bought, the receiver and software are made in house.";
			}
			#productIntegration action integrateProducts {
				:>> tailoring = "Applied as written at the subsystem and instrument layers.";
			}
			#productVerification action verifyProducts {
				:>> tailoring = "Applied as written; verification by test at the instrument layer, by analysis below it.";
			}
			#productValidation action validateProducts {
				:>> tailoring = "Applied as written; validation against the science team's expectations.";
			}
			#productTransition action transitionProducts {
				:>> tailoring = "Applied as written for delivery to the host spacecraft.";
			}
			#technicalPlanning action planTechnicalEffort {
				:>> tailoring = "Applied as written; this SEMP is the plan.";
			}
			#requirementsManagement action manageRequirements {
				:>> tailoring = "Customized: the model is the requirements database.";
			}
			#interfaceManagement action manageInterfaces {
				:>> tailoring = "Applied as written for the spacecraft interfaces.";
			}
			#technicalRiskManagement action manageRisks {
				:>> tailoring = "Applied as written with a monthly risk board.";
			}
			#configurationManagement action manageConfiguration {
				:>> tailoring = "Applied as written; the model repository is the configuration baseline.";
			}
			#technicalDataManagement action manageTechnicalData {
				:>> tailoring = "Applied as written.";
			}
			#technicalAssessment action assessProgress {
				:>> tailoring = "Applied as written through the TPMs below and the life-cycle reviews.";
			}
			#decisionAnalysis action analyseDecisions {
				:>> tailoring = "Customized: trade studies are recorded as analysis cases in the model.";
			}
		}

		#semp part sempOutline : SEMP {
			doc /* Deliberate gap: the first outline of the SEMP addresses only the system design processes,
			 * not all 17 (SE-58). */
			:>> maturity = WorkProductMaturity::preliminary;
			#stakeholderExpectationsDefinition action defineStakeholderExpectations;
			#technicalRequirementsDefinition action defineRequirements;
			#logicalDecomposition action decomposeLogically;
			#designSolutionDefinition action defineDesignSolution;
		}

		#mop attribute massMargin : Real {
			doc /* Mass margin, the leading indicator SE-62 asks for. */
		}
		#mop attribute powerMargin : Real {
			doc /* Power margin, the leading indicator SE-63 asks for. */
		}
		#moe attribute dustDetectionRate : Real {
			doc /* Fraction of dust events above threshold the instrument detects. */
		}
	}

	package 'Work Products' {
		doc /* The section 5.2.2.2 minimum products the project has produced so far, at their maturity. */

		#stakeholderIdentificationAndExpectationsDefinition item expectations {
			:>> maturity = WorkProductMaturity::baseline;
		}
		#conceptDefinition item concept {
			:>> maturity = WorkProductMaturity::baseline;
		}
		#measureOfEffectivenessDefinition item moeDefinition {
			:>> maturity = WorkProductMaturity::approve;
		}
		#requirementSet item requirements {
			:>> maturity = WorkProductMaturity::baseline;
		}
		#humanSystemsIntegrationApproach item hsiApproach {
			:>> maturity = WorkProductMaturity::baseline;
		}
		#designSolution item preliminaryDesign {
			:>> maturity = WorkProductMaturity::preliminary;
		}
		#integrationPlan item integrationPlanDraft {
			:>> maturity = WorkProductMaturity::baseline;
		}
		#verificationAndValidationPlan item vvPlan {
			doc /* Still preliminary at PDR, below the baseline section 5.2.2.2.d(3) asks for. */
			:>> maturity = WorkProductMaturity::preliminary;
		}
	}

	package 'Life-Cycle Reviews' {
		private import 'Technical Planning'::pathfinderSEMP;
		private import 'Work Products'::*;
		private import NPR7123::ReviewCriteria::*;

		#lifeCycleReview action srr {
			doc /* System Requirements Review, Phase A. */
			:>> kind = ReviewKind::SRR;
			:>> phase = LifeCyclePhase::phaseA;
			:>> status = StatusKind::done;

			requirement entrance1 : SRR::Entrance1 :> entranceCriteria;
			requirement entrance2 : SRR::Entrance2 :> entranceCriteria {
				doc /* Tailored: the review board is the project's standing engineering review board. */
			}
			requirement success1 : SRR::Success1 :> successCriteria;
			requirement success2 : SRR::Success2 :> successCriteria;
			requirement success3 : SRR::Success3 :> successCriteria;

			#semp part sempAtSRR :> reviewedProducts {
				:>> maturity = WorkProductMaturity::baseline;
			}
			#requirementSet item requirementsAtSRR :> reviewedProducts {
				:>> maturity = WorkProductMaturity::baseline;
			}
			#humanSystemsIntegrationApproach item hsiAtSRR :> reviewedProducts {
				:>> maturity = WorkProductMaturity::baseline;
			}

			#reviewItem item rfa1 {
				:>> kind = ReviewItemKind::requestForAction;
				:>> status = StatusKind::closed;
			}
		}

		#lifeCycleReview action pdr {
			doc /* Preliminary Design Review, Phase B. */
			:>> kind = ReviewKind::PDR;
			:>> phase = LifeCyclePhase::phaseB;
			:>> status = StatusKind::open;

			requirement entrance1 : PDR::Entrance1 :> entranceCriteria;
			requirement entrance3 : PDR::Entrance3 :> entranceCriteria;
			requirement success1 : PDR::Success1 :> successCriteria;
			requirement success2 : PDR::Success2 :> successCriteria {
				doc /* Tailored: cost and schedule credibility is assessed by the host mission's PDR board. */
			}

			#designSolution item designAtPDR :> reviewedProducts {
				:>> maturity = WorkProductMaturity::preliminary;
			}
			#integrationPlan item integrationPlanAtPDR :> reviewedProducts {
				:>> maturity = WorkProductMaturity::baseline;
			}
			#verificationAndValidationPlan item vvPlanAtPDR :> reviewedProducts {
				doc /* Deliberate gap: section 5.2.2.2.d(3) asks for the V&V Plan baselined at PDR. */
				:>> maturity = WorkProductMaturity::preliminary;
			}
		}

		#lifeCycleReview action cdr {
			doc /* Deliberate gap: the CDR is planned but states no entrance or success criteria yet, and no
			 * product, so the baseline detailed design section 5.2.2.2.e asks for is not stated. */
			:>> kind = ReviewKind::CDR;
			:>> phase = LifeCyclePhase::phaseC;
		}

		#peerReview action lidarSoftwarePeerReview {
			doc /* Peer review of the receiver software design (Table G-19). */
			requirement entrance1 : PeerReview::Entrance1 :> entranceCriteria;
			requirement success1 : PeerReview::Success1 :> successCriteria;
		}

		#keyDecisionPoint occurrence kdpC {
			:>> designation = "KDP C";
			:>> phase = LifeCyclePhase::phaseB;
		}
	}

	package Instrument {
		part def DustLidar {
			part transmitter {
				@TechnologyReadiness {
					level = TechnologyReadinessLevel::trl6;
					assessedAt = ReviewKind::PDR;
					basis = "Prototype transmitter demonstrated in thermal vacuum.";
				}
			}
			part receiver;
		}
		part lidar : DustLidar;
	}

	package 'Compliance Entries' {
		doc /* The project's compliance with each active SE requirement; the matrix below generates
		 * Table H-1 from these entries. SE-31 is left out on purpose. */

		part complianceEntries {

			requirement se06 : SE06 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se07 : SE07 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se08 : SE08 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se09 : SE09 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se10 : SE10 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se11 : SE11 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se12 : SE12 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se13 : SE13 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se14 : SE14 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se15 : SE15 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se16 : SE16 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se17 : SE17 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se18 : SE18 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se19 : SE19 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se20 : SE20 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se21 : SE21 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se22 : SE22 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se23 : SE23 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se24 : SE24 {
				@Compliance { comply = ComplyKind::notApplicable; justification = "The instrument is built in house; no contract is let."; }
			}
			requirement se25 : SE25 {
				@Compliance { comply = ComplyKind::notApplicable; justification = "No contract is let."; }
			}
			requirement se26 : SE26 {
				@Compliance { comply = ComplyKind::notApplicable; justification = "No contract is let."; }
			}
			requirement se27 : SE27 {
				@Compliance { comply = ComplyKind::notApplicable; justification = "No contract is let."; }
			}
			requirement se28 : SE28 {
				@Compliance { comply = ComplyKind::notApplicable; justification = "No contract is let."; }
			}
			requirement se29 : SE29 {
				@Compliance { comply = ComplyKind::notApplicable; justification = "No contract is let."; }
			}
			requirement se30 : SE30 {
				@Compliance { comply = ComplyKind::notApplicable; justification = "No contract is let."; }
			}
			requirement se32 : SE32 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se33 : SE33 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se34 : SE34 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se35 : SE35 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se36 : SE36 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se37 : SE37 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se38 : SE38 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se39 : SE39 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se40 : SE40 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se41 : SE41 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se42 : SE42 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se43 : SE43 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se44 : SE44 {
				@Compliance { comply = ComplyKind::notApplicable; justification = "The project is a single project, not a program."; }
			}
			requirement se45 : SE45 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se46 : SE46 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se47 : SE47 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se48 : SE48 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se51 : SE51 {
				@Deviation {
					requirementId = "SE-51";
					rationale = "Decommissioning and disposal plans are the host spacecraft's.";
					approvedBy = "Center Engineering Technical Authority";
				}
				@Compliance { comply = ComplyKind::tailored; justification = "Decommissioning is the host spacecraft's; the project delivers inputs to its plan (deviation D-3)."; }
			}
			requirement se52 : SE52 {
				@Compliance { comply = ComplyKind::tailored; justification = "Disposal is the host spacecraft's; the project delivers inputs to its plan (deviation D-3)."; }
			}
			requirement se53 : SE53 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se54 : SE54 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se55 : SE55 {
				@Compliance { comply = ComplyKind::tailored; justification = "Decommissioning is the host spacecraft's (deviation D-3)."; }
			}
			requirement se56 : SE56 {
				@Compliance { comply = ComplyKind::tailored; justification = "Disposal is the host spacecraft's (deviation D-3)."; }
			}
			requirement se57 : SE57 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se58 : SE58 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se59 : SE59 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se60 : SE60 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se61 : SE61 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se62 : SE62 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se63 : SE63 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se64 : SE64 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se65 : SE65 {
				@Compliance { comply = ComplyKind::tailored; }
				doc /* Deliberate gap: tailored with no justification. */
			}
			requirement se66 : SE66 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se67 : SE67 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se68 : SE68 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}
			requirement se69 : SE69 {
				@Compliance { comply = ComplyKind::fullyCompliant; }
			}

			#technicalAuthority part eta : TechnicalAuthority {
				:>> kind = TechnicalAuthorityKind::engineering;
				:>> holder = "Center Chief Engineer";
			}
		}
	}

	package 'Compliance Matrix' {
		private import 'Compliance Entries'::complianceEntries;

		part def 'Pathfinder Compliance Matrix' :> ComplianceMatrix {
			:>> title = "Dust Lidar Pathfinder NPR 7123.1 Compliance Matrix";

			part scope : Paragraph {
				:>> text = "Table H-1 of NPR 7123.1D for the Dust Lidar Pathfinder, generated from the compliance entries of the project model.";
			}

			part entries : Section {
				:>> title = "Compliance matrix";
				part matrix : Table {
					:>> caption = "Compliance with the active SE requirements";
					calc rows : ComplianceRegister { in :>> root = complianceEntries; }
				}
			}
		}

		part complianceMatrix : 'Pathfinder Compliance Matrix';
	}
}
