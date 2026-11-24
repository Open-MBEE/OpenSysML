/*
 * Lunar Night Survival Heater: a small fictional use of a model and simulation worked through
 * NASA-STD-7009B with the NASASTD7009 library. A lumped thermal model of an electronics box sizes
 * the heater that keeps it alive through the lunar night; the model states the criticality of
 * that decision, the M&S capability and M&S results assessments, the report of the results to
 * decision makers and the Requirements Identification Matrix. The gaps the NASASTD7009 checks
 * report are left open on purpose and marked with a doc comment.
 */
package 'Lunar Night Survival Heater' {
	private import NASASTD7009::*;
	private import NASASTD7009::MSRequirements::*;
	private import NASASTD7009::MSRecommendations::*;
	private import ScalarValues::*;
	private import DocumentQueries::*;

	package 'Thermal Model' {
		doc /* The model and the analysis that uses it. The criticality assessment is the record
		 * [M&S 6] asks for, on the analysis whose result informs the decision. */

		calc def HeaterPower {
			doc /* Steady-state heater power, in watts, holding a box at a temperature against its
			 * conductive and radiative loss to the lunar-night sink. */
			in conductance : Real;
			in radiativeCoupling : Real;
			in boxTemperature : Real;
			in sinkTemperature : Real;
			return power : Real = conductance * (boxTemperature - sinkTemperature)
				+ radiativeCoupling * (boxTemperature ** 4 - sinkTemperature ** 4);
		}

		analysis def 'Heater Sizing' {
			doc /* Sizes the survival heater of the electronics box. The decision it informs is the
			 * heater and battery allocation; no test of the flight box in lunar night exists yet,
			 * but the vendor's qualification test of a similar box does. */
			@CriticalityAssessment {
				consequence = DecisionConsequence::significant;
				influence = MSInfluence::significant;
				rating = CriticalityRating::red;
				basis = "An undersized heater loses the instrument; the vendor qualification test of a similar box is the only other evidence.";
			}

			attribute conductance : Real = 0.05;
			attribute radiativeCoupling : Real = 2.0e-9;
			attribute survivalTemperature : Real = 233.0;
			attribute sinkTemperature : Real = 100.0;
			attribute margin : Real = 1.25;

			return heaterPower : Real = margin * HeaterPower(conductance, radiativeCoupling, survivalTemperature, sinkTemperature);
		}

		analysis heaterSizing : 'Heater Sizing';

		analysis def 'Harness Loss Estimate' {
			doc /* Deliberate gap: Figure 1 rates a moderate consequence with a controlling influence
			 * red, and the assessment records no method of its own. */
			@CriticalityAssessment {
				consequence = DecisionConsequence::moderate;
				influence = MSInfluence::controlling;
				rating = CriticalityRating::yellow;
			}
		}
	}

	package 'Capability Assessment' {
		doc /* The assessed M&S capability, by the five factors of Appendix E.3 [M&S 48], with the
		 * thresholds the project set as acceptance criteria [M&S 43] e. */

		#capabilityAssessment item heaterCapability {
			#capabilityFactor item dataPedigree {
				:>> factor = CapabilityFactor::msDataPedigree;
				:>> level = FactorLevel::level3;
				:>> threshold = FactorLevel::level2;
				:>> evidence = "Every material property is traced to the vendor's data sheets or the qualification test report.";
			}
			#capabilityFactor item modelVerification {
				:>> factor = CapabilityFactor::msVerification;
				:>> level = FactorLevel::level2;
				:>> threshold = FactorLevel::level2;
				:>> evidence = "Documented unit tests of every node and coupling; the energy balance closes to 0.1 percent.";
			}
			#capabilityFactor item modelValidation {
				:>> factor = CapabilityFactor::msValidation;
				:>> level = FactorLevel::level2;
				:>> threshold = FactorLevel::level3;
				:>> evidence = "Compared with the vendor's qualification test of a similar box; the flight box is untested.";
			}
			#capabilityFactor item developmentReview {
				:>> factor = CapabilityFactor::msDevelopmentTechnicalReview;
				:>> level = FactorLevel::level2;
				:>> threshold = FactorLevel::level2;
				:>> evidence = "Formal internal peer review of the data, the verification and the validation.";
			}
			#capabilityFactor item developmentManagement {
				:>> factor = CapabilityFactor::msDevelopmentProcessProductManagement;
				:>> level = FactorLevel::level2;
				:>> threshold = FactorLevel::level2;
				:>> evidence = "The model is under configuration management with an approved development plan.";
			}
		}
	}

	package 'Results Assessment' {
		doc /* The assessed M&S results of the heater sizing, by the six factors of Appendix E.4
		 * [M&S 31]. */

		#resultsAssessment item heaterResults {
			#resultsFactor item useAssessment {
				:>> factor = ResultsFactor::msUseAssessment;
				:>> level = FactorLevel::level2;
				:>> threshold = FactorLevel::level2;
				:>> evidence = "The proposed use is within the model's limits; it differs moderately from the vendor's use.";
			}
			#resultsFactor item inputPedigree {
				:>> factor = ResultsFactor::msInputPedigree;
				:>> level = FactorLevel::level3;
				:>> threshold = FactorLevel::level2;
				:>> evidence = "Sink temperature and couplings are traced to the lander's thermal environment document.";
			}
			#resultsFactor item uncertaintyCharacterization {
				:>> factor = ResultsFactor::msUncertaintyCharacterization;
				:>> level = FactorLevel::level2;
				:>> threshold = FactorLevel::level2;
				:>> evidence = "Coupling and sink-temperature uncertainties are bounded and propagated by boundary analysis.";
			}
			#resultsFactor item resultsRobustness {
				:>> factor = ResultsFactor::msResultsRobustness;
				:>> level = FactorLevel::level2;
				:>> threshold = FactorLevel::level2;
				:>> evidence = "Sensitivity to the conductance and the sink temperature is quantified by a sweep.";
			}
			#resultsFactor item useReview {
				:>> factor = ResultsFactor::msUseAnalysisTechnicalReview;
				:>> level = FactorLevel::level1;
				:>> threshold = FactorLevel::level1;
				:>> evidence = "Informal internal peer review of the inputs, the uncertainty and the sweep.";
			}
			#resultsFactor item useManagement {
				:>> factor = ResultsFactor::msUseProcessProductManagement;
				:>> level = FactorLevel::level2;
				:>> threshold = FactorLevel::level2;
				:>> evidence = "The analysis follows the approved analysis procedure; inputs and results are under configuration management.";
			}
		}

		#resultsAssessment item earlyResults {
			doc /* Deliberate gap: the assessment made before the sweep records two of the six factors. */
			#resultsFactor item useAssessment {
				:>> factor = ResultsFactor::msUseAssessment;
				:>> level = FactorLevel::level2;
			}
			#resultsFactor item inputPedigree {
				:>> factor = ResultsFactor::msInputPedigree;
				:>> level = FactorLevel::level3;
			}
		}
	}

	package 'Results Report' {
		doc /* What section 4.3.8 has reported to decision makers with the results. */

		private import 'Capability Assessment'::heaterCapability;
		private import 'Results Assessment'::heaterResults;

		calc def ReportedWarnings :> Query {
			doc /* The warnings [M&S 32] has accompany the results: one row per ResultsWarning the
			 * report nests, with the kind of occurrence and its impact.
			 */
			Project(
				source = WhereType(source = Descendants(source = Named(qualifiedName = "'Lunar Night Survival Heater'::'Results Report'::'Heater Sizing Report'")), type = "NASASTD7009::ResultsWarning"),
				properties = ("name", "kind", "impact"),
				tolerate = true
			)
		}

		calc def ReportStatements :> Query {
			doc /* What sections 4.3.8.2 to 4.3.8.6 have a report state with the results: the estimate of
			 * uncertainty and its process, the review findings, the qualifications and the risk
			 * acceptance of the report at root.
			 */
			in root : KerML::Root::Element;
			Project(
				source = root,
				properties = ("uncertaintyEstimate", "uncertaintyProcess", "technicalReviewFindings", "qualifications", "riskAcceptance"),
				tolerate = true
			)
		}

		part def 'Heater Sizing Report' :> MSResultsReport {
			:>> title = "Lunar Night Survival Heater Sizing: Report of M&S Results";
			:>> uncertaintyEstimate = UncertaintyEstimateKind::quantitative;
			:>> uncertaintyProcess = "Boundary analysis over the bounded coupling and sink-temperature uncertainties.";
			:>> technicalReviewFindings = "The peer reviews found the validation referent adequate for sizing and asked for a test of the flight box.";
			:>> qualifications = "The model's developer and the analyst are thermal engineers with flight thermal-model experience and the project's M&S training.";
			:>> riskAcceptance = "The project accepts the validation gap with a 25 percent power margin until the flight box is tested.";

			item capability :> heaterCapability;
			item results :> heaterResults;

			#resultsWarning item validationGap {
				:>> kind = WarningOccurrence::unachievedAcceptanceCriteria;
				:>> impact = "The validation factor is one level below its threshold: the heater power carries the uncertainty of a similar, not the flight, box.";
			}

			part outcome : Section {
				:>> title = "Assessment outcomes";
				part capabilityOutcome : Table {
					:>> caption = "M&S capability assessment";
					calc rows : CapabilityAssessmentRegister { in :>> root = heaterCapability; }
				}
				part resultsOutcome : Table {
					:>> caption = "M&S results assessment";
					calc rows : ResultsAssessmentRegister { in :>> root = heaterResults; }
				}
			}

			part warnings : Section {
				:>> title = "Warnings";
				part explicitWarnings : Table {
					:>> caption = "Explicit warnings accompanying the results [M&S 32]";
					calc rows : ReportedWarnings;
				}
			}

			part statements : Section {
				:>> title = "Uncertainty, reviews, qualifications and risk";
				part reportStatements : Table {
					:>> caption = "What the report states with the results [M&S 33], [M&S 34], [M&S 36], [M&S 37], [M&S 39]";
					calc rows : ReportStatements { in :>> root = heaterSizingReport; }
				}
			}
		}

		part heaterSizingReport : 'Heater Sizing Report';

		#resultsReport part draftReport {
			doc /* Deliberate gap: the draft states the uncertainty estimate but not the process that
			 * obtained it, the qualifications or the risk acceptance; it assesses validation below
			 * its threshold with no warning of unachieved acceptance criteria; and its one warning
			 * states no impact. */
			:>> uncertaintyEstimate = UncertaintyEstimateKind::qualitative;

			item capability :> heaterCapability;
			item results :> heaterResults;

			#resultsWarning item sinkTemperatureLimit {
				:>> kind = WarningOccurrence::violatedLimits;
			}
		}
	}

	package 'Matrix Entries' {
		doc /* One entry per requirement of the Standard, as the Requirements Identification Matrix
		 * of Appendix A asks, and the recommendations the project adopts (Appendix B). */

		part matrixEntries {
			doc /* Deliberate gap: no entry covers [M&S 51], the record of M&S defects or problems. */

			requirement ms40 : MS40 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/README.md, section Intended use"; }
			}
			requirement ms6 : MS6 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterSizing"; }
			}
			requirement ms41 : MS41 {
				@Applicability { applicable = Applicable::yes; recordLocation = "docs/thermal-model-plan.md"; }
			}
			requirement ms42 : MS42 {
				doc /* Deliberate gap: an applicable requirement with no record location. */
				@Applicability { applicable = Applicable::yes; }
			}
			requirement ms43 : MS43 {
				@Applicability { applicable = Applicable::yes; recordLocation = "docs/thermal-model-plan.md, section Acceptance criteria"; }
			}
			requirement ms44 : MS44 {
				@Applicability { applicable = Applicable::yes; recordLocation = "docs/thermal-model-plan.md, section Reporting"; }
			}
			requirement ms9 : MS9 {
				@Applicability { applicable = Applicable::yes; recordLocation = "reviews/2031-thermal-peer-review.md"; }
			}
			requirement ms10 : MS10 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/README.md, section Electronics box"; }
			}
			requirement ms45 : MS45 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/data/"; }
			}
			requirement ms46 : MS46 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/README.md, section Units and frames"; }
			}
			requirement ms11 : MS11 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/README.md, section Assumptions"; }
			}
			requirement ms12 : MS12 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/README.md, section Equations"; }
			}
			requirement ms13 : MS13 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/README.md, section Limits"; }
			}
			requirement ms14 : MS14 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/README.md, section Permissible uses"; }
			}
			requirement ms48 : MS48 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterCapability"; }
			}
			requirement ms47 : MS47 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/USERS-GUIDE.md"; }
			}
			requirement ms15 : MS15 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/verification/"; }
			}
			requirement ms16 : MS16 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/verification/domain.md"; }
			}
			requirement ms17 : MS17 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/validation/"; }
			}
			requirement ms18 : MS18 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/validation/domain.md"; }
			}
			requirement ms19 : MS19 {
				@Applicability {
					applicable = Applicable::no;
					comments = "The referent is the vendor's qualification test, whose uncertainty the vendor characterized and reports.";
				}
			}
			requirement ms21 : MS21 {
				@Applicability { applicable = Applicable::yes; recordLocation = "models/thermal/README.md, section Uncertainties"; }
			}
			requirement ms22 : MS22 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/proposed-use.md"; }
			}
			requirement ms23 : MS23 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/use-assessment.md"; }
			}
			requirement ms24 : MS24 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/inputs.md"; }
			}
			requirement ms25 : MS25 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/setup.md"; }
			}
			requirement ms26 : MS26 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/use-assessment.md"; }
			}
			requirement ms27 : MS27 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/run-log.md"; }
			}
			requirement ms28 : MS28 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/uncertainty.md"; }
			}
			requirement ms29 : MS29 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/uncertainty.md"; }
			}
			requirement ms30 : MS30 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/sensitivity.md"; }
			}
			requirement ms31 : MS31 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterResults"; }
			}
			requirement ms49 : MS49 {
				@Applicability { applicable = Applicable::yes; recordLocation = "analyses/heater-sizing/risk.md"; }
			}
			requirement ms32 : MS32 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterSizingReport"; }
			}
			requirement ms33 : MS33 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterSizingReport"; }
			}
			requirement ms34 : MS34 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterSizingReport"; }
			}
			requirement ms50 : MS50 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterSizingReport"; }
			}
			requirement ms35 : MS35 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterSizingReport"; }
			}
			requirement ms36 : MS36 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterSizingReport"; }
			}
			requirement ms37 : MS37 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterSizingReport"; }
			}
			requirement ms38 : MS38 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, requirementsMatrix"; }
			}
			requirement ms39 : MS39 {
				@Applicability { applicable = Applicable::yes; recordLocation = "this model, heaterSizingReport"; }
			}
		}

		part recommendationEntries {
			requirement r4_1_4a : R4_1_4a {
				@Applicability { applicable = Applicable::yes; }
			}
			requirement r4_2_2g : R4_2_2g {
				@Applicability { applicable = Applicable::yes; }
			}
			requirement r4_3_2b : R4_3_2b {
				@Applicability { applicable = Applicable::yes; }
			}
			requirement r4_3_9c : R4_3_9c {
				@Applicability {
					applicable = Applicable::no;
					comments = "Tailored: results are kept in the analysis repository, which the configuration management plan covers.";
				}
			}
		}
	}

	package 'Requirements Matrix' {
		private import 'Matrix Entries'::matrixEntries;
		private import 'Matrix Entries'::recommendationEntries;

		part def 'Heater Requirements Identification Matrix' :> RequirementsIdentificationMatrix {
			:>> title = "Lunar Night Survival Heater NASA-STD-7009B Requirements Identification Matrix";

			part scope : Paragraph {
				:>> text = "Table 1 of NASA-STD-7009B for the survival heater thermal model, generated from the matrix entries of the project model.";
			}

			part entries : Section {
				:>> title = "Requirements identification matrix";
				part matrix : Table {
					:>> caption = "Applicability of the requirements of NASA-STD-7009B";
					calc rows : RequirementsIdentificationRegister { in :>> root = matrixEntries; }
				}
			}
		}

		part requirementsMatrix : 'Heater Requirements Identification Matrix';

		part def 'Heater Recommendations Adoption Matrix' :> RecommendationsAdoptionMatrix {
			:>> title = "Lunar Night Survival Heater NASA-STD-7009B Recommendations Adoption Matrix";

			part entries : Section {
				:>> title = "Recommendations adoption matrix";
				part matrix : Table {
					:>> caption = "Adoption of the recommendations of NASA-STD-7009B";
					calc rows : RecommendationsAdoptionRegister { in :>> root = recommendationEntries; }
				}
			}
		}

		part recommendationsMatrix : 'Heater Recommendations Adoption Matrix';
	}
}
