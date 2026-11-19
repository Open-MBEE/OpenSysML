library package NPR7123 {
	doc /* NON-NORMATIVE OpenSysML vocabulary for NASA Procedural Requirements NPR 7123.1D, 'NASA
	 * Systems Engineering Processes and Requirements' (Office of the Chief Engineer, effective
	 * 2023-07-05, expires 2028-07-05), published in the NASA Online Directives Information System
	 * (NODIS) at
	 * https://nodis3.gsfc.nasa.gov/displayDir.cfm?Internal_ID=N_PR_7123_001D_&page_name=main. The
	 * NPR is a work of the United States Government; the library quotes its text verbatim where the
	 * NPR defines a term, with the section, table or requirement identifier the text comes from.
	 * The library is a community extension: it is not endorsed by NASA and it does not replace the
	 * NPR, the governing program/project management NPR (NPR 7120.5, NPR 7120.7, NPR 7120.8) or a
	 * Center's implementing procedures. It lets a project model, in standard SysML v2, the 17
	 * common technical processes of chapter 3 and their SEMP tailoring (SE-58), the life-cycle and
	 * technical reviews of chapter 5 with the entrance and success criteria of Appendix G, the
	 * minimum work products of section 5.2.2.2 and the work-product maturity of Table 5-1 and
	 * Appendix F, the technology readiness levels of Appendix E, the active [SE-nn] requirements
	 * and the compliance matrix of Appendix H (Table H-1), and the Technical Authority, waiver and
	 * deviation vocabulary of chapter 2. Each artefact gets a definition specialising its native
	 * SysML type, an abstract base usage subsetting the native base usage, and a semantic-metadata
	 * keyword applying both at once: `#stakeholderExpectationsDefinition action elicit;`,
	 * `#lifeCycleReview action pdr { :>> kind = ReviewKind::PDR; }`. Measures of effectiveness,
	 * technical performance measures and leading indicators reuse the standard
	 * ParametersOfInterestMetadata library (`#moe`, `#mop`), which is re-exported, as is
	 * ModelingMetadata for status and rationale.
	 */

	private import ScalarValues::Boolean;
	private import ScalarValues::Integer;
	private import ScalarValues::String;
	private import Metaobjects::SemanticMetadata;
	private import KerML::Root::Element;
	private import Actions::Action;
	private import Actions::actions;
	private import Occurrences::Occurrence;
	private import Occurrences::occurrences;
	private import Items::Item;
	private import Items::items;
	private import Parts::Part;
	private import Parts::parts;
	private import Requirements::RequirementCheck;
	private import Requirements::requirementChecks;
	private import StandardViewDefinitions::*;
	private import Views::asElementTable;
	private import DocumentQueries::*;

	public import ParametersOfInterestMetadata::*;
	public import ModelingMetadata::*;

	/* Chapter 3: the 17 common technical processes of the SE engine */

	enum def TechnicalProcessGroup {
		doc /* The three groups of common technical processes of NPR 7123.1D section 3.1.1 and Figure
		 * 3-1: 'a. System design processes for top-down design of each product in the system
		 * structure. b. Product realization processes for bottom-up realization of each product in
		 * the system structure. c. Cross-cutting technical management processes for planning,
		 * assessing, and controlling the implementation of the system design and product
		 * realization processes and to guide technical decision making (decision analysis).'
		 */
		systemDesign;
		productRealization;
		technicalManagement;
	}

	abstract action def TechnicalProcess :> Action {
		doc /* One of the 17 common technical processes of NPR 7123.1D chapter 3, which the NPR calls
		 * the 'SE engine' (section 3.1.2): 'these common technical processes are used to drive the
		 * development of the system products and associated work products required by management to
		 * satisfy the applicable product life-cycle phase success criteria while meeting
		 * stakeholder expectations within cost, schedule, and risk constraints.' Each process
		 * definition below records its group, the section describing it and the [SE-nn] requirement
		 * that asks Program/Project Managers to identify and implement an ETA-approved process. A
		 * project states in its SEMP how each process, as tailored, is applied (SE-58): nest one
		 * usage per process under the SEMP and describe the tailoring in `tailoring`.
		 */
		attribute group : TechnicalProcessGroup;
		attribute requirementId : String;
		attribute section : String;
		attribute tailoring : String[0..1] {
			doc /* How the project tailors or customizes the process (sections 2.2 and 6.2.3): tailoring
			 * 'is adjusting or seeking relief from requirements by use of waiver or deviation',
			 * customizing 'is selecting how the requirement is implemented and does not require
			 * waivers or deviations'.
			 */
		}
	}

	abstract action technicalProcesses : TechnicalProcess[0..*] nonunique :> actions;

	action def StakeholderExpectationsDefinition :> TechnicalProcess {
		doc /* Stakeholder Expectations Definition process (NPR 7123.1D section 3.2.2, requirement
		 * SE-07): The Stakeholder Expectations Definition process is used to elicit and define use
		 * cases, scenarios, concept of operations, and stakeholder expectations for the applicable
		 * product life-cycle phases and product layer. This includes expectations such as: a.
		 * Operational end products and life-cycle-enabling products of the product layer. b.
		 * Affordability. c. Operator or user interfaces. d. Expected skills and capabilities of
		 * operators or users. e. Expected number of simultaneous users. f. System and human
		 * performance criteria. g. Technical authority, standards, regulations, and laws. h.
		 * Factors such as health and medical, safety, planetary protection, orbital debris,
		 * quality, system security (including cybersecurity), physical security, context of human
		 * interactions, robustness, resilience, reliability, availability, integrity of data and
		 * processes, maintainability, electromagnetic compatibility, interoperability, testability,
		 * transportability, supportability, usability, and disposability. i. For crewed missions,
		 * crew health and performance capabilities and limitations, risk posture, crew
		 * survivability, and system habitability. j. Local management constraints on how work will
		 * be done (e.g., operating procedures). The baselined stakeholder expectations are used for
		 * validation of the product layer end product during product realization. At this point,
		 * Measures of Effectiveness (MOEs) are defined. For more information of MOEs refer to
		 * NASA/SP-6105.
		 */
		:>> group = TechnicalProcessGroup::systemDesign;
		:>> requirementId = "SE-07";
		:>> section = "3.2.2";
	}

	abstract action stakeholderExpectationsDefinitions : StakeholderExpectationsDefinition[0..*] nonunique :> technicalProcesses;

	action def TechnicalRequirementsDefinition :> TechnicalProcess {
		doc /* Technical Requirements Definition process (NPR 7123.1D section 3.2.3, requirement SE-08):
		 * The technical requirements definition process is used to transform the baselined
		 * stakeholder expectations into unique, quantitative, and measurable technical requirements
		 * expressed as "shall" statements that can be used for defining a design solution for the
		 * product layer end product and related enabling products. The technical team ensures that
		 * system security requirement (including cybersecurity requirements that derive from NPR
		 * 2810.1) are considered in the technical requirements. This process also includes
		 * validation of the requirements to ensure that the requirements are well-formed (clear and
		 * unambiguous), complete (agrees with customer and stakeholder needs and expectations),
		 * consistent (conflict free), and individually verifiable and traceable to a higher level
		 * requirement or goal. As part of this process, Measures of Performance (MOPs) and
		 * Technical Performance Measures (TPMs) are defined. For more information of MOPs and TPMs,
		 * refer to NASA/SP-6105.
		 */
		:>> group = TechnicalProcessGroup::systemDesign;
		:>> requirementId = "SE-08";
		:>> section = "3.2.3";
	}

	abstract action technicalRequirementsDefinitions : TechnicalRequirementsDefinition[0..*] nonunique :> technicalProcesses;

	action def LogicalDecomposition :> TechnicalProcess {
		doc /* Logical Decomposition process (NPR 7123.1D section 3.2.4, requirement SE-09): The logical
		 * decomposition process is used to improve understanding of the defined technical
		 * requirements and the relationships among the requirements (e.g., functional, behavioral,
		 * performance, and temporal) and to transform the defined set of technical requirements
		 * into a set of logical decomposition models and their associated set of derived technical
		 * requirements for lower levels of the system and for input to the design solution
		 * definition process.
		 */
		:>> group = TechnicalProcessGroup::systemDesign;
		:>> requirementId = "SE-09";
		:>> section = "3.2.4";
	}

	abstract action logicalDecompositions : LogicalDecomposition[0..*] nonunique :> technicalProcesses;

	action def DesignSolutionDefinition :> TechnicalProcess {
		doc /* Design Solution Definition process (NPR 7123.1D section 3.2.5, requirement SE-10): The
		 * Design Solution Definition process is used to translate the outputs of the logical
		 * decomposition process into a design solution definition that is in a form consistent with
		 * the product life-cycle phase and product layer location in the system structure and that
		 * will satisfy phase success criteria. This includes transforming the defined logical
		 * decomposition models and their associated sets of derived technical requirements into
		 * alternative solutions, then analyzing each alternative to be able to select a preferred
		 * alternative and fully defining that alternative into a final design solution definition
		 * that will satisfy the requirements. These design solution definitions will be used for
		 * generating end products, either by using the product implementation process or product
		 * integration process, as a function of the position of the product layer in the system
		 * structure and whether there are additional subsystems of the end product that need to be
		 * defined. The output definitions from the design solution (end product specifications)
		 * will be used for conducting product verification.
		 */
		:>> group = TechnicalProcessGroup::systemDesign;
		:>> requirementId = "SE-10";
		:>> section = "3.2.5";
	}

	abstract action designSolutionDefinitions : DesignSolutionDefinition[0..*] nonunique :> technicalProcesses;

	action def ProductImplementation :> TechnicalProcess {
		doc /* Product Implementation process (NPR 7123.1D section 3.2.6, requirement SE-11): The
		 * Product Implementation Process is used to generate a specified product of a product layer
		 * through buying, making, or reusing in a form consistent with the product life-cycle phase
		 * success criteria and that satisfies the design solution definition (e.g., drawings,
		 * specifications).
		 */
		:>> group = TechnicalProcessGroup::productRealization;
		:>> requirementId = "SE-11";
		:>> section = "3.2.6";
	}

	abstract action productImplementations : ProductImplementation[0..*] nonunique :> technicalProcesses;

	action def ProductIntegration :> TechnicalProcess {
		doc /* Product Integration process (NPR 7123.1D section 3.2.7, requirement SE-12): The Product
		 * Integration Process is used to transform lower level, verified and validated end products
		 * into the desired end product of the higher level product layer through assembly and
		 * integration.
		 */
		:>> group = TechnicalProcessGroup::productRealization;
		:>> requirementId = "SE-12";
		:>> section = "3.2.7";
	}

	abstract action productIntegrations : ProductIntegration[0..*] nonunique :> technicalProcesses;

	action def ProductVerification :> TechnicalProcess {
		doc /* Product Verification process (NPR 7123.1D section 3.2.8, requirement SE-13): The Product
		 * Verification process is used to demonstrate that an end product generated from product
		 * implementation or product integration conforms to its requirements as a function of the
		 * product life-cycle phase and the location of the product layer end product in the system
		 * structure. Special attention is given to demonstrating satisfaction of the MOPs defined
		 * for each MOE during performance of the technical requirements definition process.
		 */
		:>> group = TechnicalProcessGroup::productRealization;
		:>> requirementId = "SE-13";
		:>> section = "3.2.8";
	}

	abstract action productVerifications : ProductVerification[0..*] nonunique :> technicalProcesses;

	action def ProductValidation :> TechnicalProcess {
		doc /* Product Validation process (NPR 7123.1D section 3.2.9, requirement SE-14): The Product
		 * Validation process is used to confirm that a verified end product generated by product
		 * implementation or product integration fulfills (satisfies) its intended use when placed
		 * in its intended environment and to ensure that any anomalies discovered during validation
		 * are appropriately resolved prior to delivery of the product (if validation is done by the
		 * supplier of the product) or prior to integration with other products into a higher level
		 * assembled product (if validation is done by the receiver of the product). The validation
		 * is done against the set of baselined stakeholder expectations. Special attention should
		 * be given to demonstrating satisfaction of the MOEs identified during performance of the
		 * stakeholder expectations definition process. The type of product validation is a function
		 * of the form of the product and product life-cycle phase and in accordance with an
		 * applicable customer agreement.
		 */
		:>> group = TechnicalProcessGroup::productRealization;
		:>> requirementId = "SE-14";
		:>> section = "3.2.9";
	}

	abstract action productValidations : ProductValidation[0..*] nonunique :> technicalProcesses;

	action def ProductTransition :> TechnicalProcess {
		doc /* Product Transition process (NPR 7123.1D section 3.2.10, requirement SE-15): The Product
		 * Transition process is used to transition a verified and validated end product that has
		 * been generated by product implementation or product integration to the customer at the
		 * next level in the system structure for integration into an end product or, for the top-
		 * level end product, transitioned to the intended end user. The form of the product
		 * transitioned will be a function of the product life-cycle phase and the location within
		 * the system structure of the product layer in which the end product exists.
		 */
		:>> group = TechnicalProcessGroup::productRealization;
		:>> requirementId = "SE-15";
		:>> section = "3.2.10";
	}

	abstract action productTransitions : ProductTransition[0..*] nonunique :> technicalProcesses;

	action def TechnicalPlanning :> TechnicalProcess {
		doc /* Technical Planning process (NPR 7123.1D section 3.2.11, requirement SE-16): The Technical
		 * Planning process is used to plan for the application and management of each common
		 * technical process, including tailoring of organizational requirements and requirements
		 * specified in this NPR. It is also used to identify, define, and plan the technical effort
		 * applicable to the product life-cycle phase for product layer location within the system
		 * structure and to meet program/project objectives and product life-cycle phase success
		 * criteria. A key document generated by this process is the SEMP (See Chapter 6).
		 */
		:>> group = TechnicalProcessGroup::technicalManagement;
		:>> requirementId = "SE-16";
		:>> section = "3.2.11";
	}

	abstract action technicalPlannings : TechnicalPlanning[0..*] nonunique :> technicalProcesses;

	action def RequirementsManagement :> TechnicalProcess {
		doc /* Requirements Management process (NPR 7123.1D section 3.2.12, requirement SE-17): The
		 * Requirements Management process is used to: a. Manage the product requirements
		 * identified, baselined, and used in the definition of the product layer products during
		 * system design. b. Provide bidirectional traceability back to the top product layer
		 * requirements. c. Manage the changes to established requirement baselines over the life
		 * cycle of the system products.
		 */
		:>> group = TechnicalProcessGroup::technicalManagement;
		:>> requirementId = "SE-17";
		:>> section = "3.2.12";
	}

	abstract action requirementsManagements : RequirementsManagement[0..*] nonunique :> technicalProcesses;

	action def InterfaceManagement :> TechnicalProcess {
		doc /* Interface Management process (NPR 7123.1D section 3.2.13, requirement SE-18): The
		 * Interface Management process is used to: a. Establish and use formal interface management
		 * to assist in controlling system product development efforts when the efforts are divided
		 * between Government programs, contractors, and/or geographically diverse technical teams
		 * within the same program or project. b. Maintain interface definition and compliance among
		 * the end products and enabling products that compose the system, as well as with other
		 * systems with which the end products and enabling products will interoperate.
		 */
		:>> group = TechnicalProcessGroup::technicalManagement;
		:>> requirementId = "SE-18";
		:>> section = "3.2.13";
	}

	abstract action interfaceManagements : InterfaceManagement[0..*] nonunique :> technicalProcesses;

	action def TechnicalRiskManagement :> TechnicalProcess {
		doc /* Technical Risk Management process (NPR 7123.1D section 3.2.14, requirement SE-19): The
		 * Technical Risk Management process is used to make risk-informed decisions and examine, on
		 * a continuing basis, the potential for deviations from the program/project plan and the
		 * consequences that could result should they occur. This enables risk-handling activities
		 * to be planned and invoked as needed across the life of the program or project to mitigate
		 * impacts on achieving product life-cycle phase success criteria and meeting technical
		 * objectives. The technical team supports the development of potential health and medical,
		 * safety, cost, and schedule impacts for identified technical risks and any associated
		 * mitigation strategies. NPR 8000.4, Agency Risk Management Procedural Requirements, is to
		 * be used as a source document for defining this process. While the focus of this process
		 * is the management of technical risk, the highly interdependent nature of health and
		 * medical, system security (including cybersecurity), safety, technical, cost, and schedule
		 * risks require the broader program/project team to consistently address risk management
		 * with an integrated approach. NASA/SP-2011-3422, NASA Risk Management Handbook, provides
		 * guidance for managing risk in an integrated fashion. in an integrated fashion.
		 */
		:>> group = TechnicalProcessGroup::technicalManagement;
		:>> requirementId = "SE-19";
		:>> section = "3.2.14";
	}

	abstract action technicalRiskManagements : TechnicalRiskManagement[0..*] nonunique :> technicalProcesses;

	action def ConfigurationManagement :> TechnicalProcess {
		doc /* Configuration Management process (NPR 7123.1D section 3.2.15, requirement SE-20): The
		 * Configuration Management process for end products, enabling products, and other work
		 * products placed under configuration control is used to: a. Identify the items to be
		 * placed under configuration control. b. Identify the configuration of the configuration
		 * item at various points in time. c. Systematically control changes to the configuration of
		 * the product or work product for the configuration item. d. Maintain the integrity,
		 * traceability, and security of the configuration item of the product or work product
		 * throughout its life. e. Preserve the records of the product or end product configuration
		 * throughout its life cycle, dispositioning them in accordance with NPR 1441.1. f. For
		 * additional information on Configuration Management refer to SAE/EIA-649, Configuration
		 * Management Standard.
		 */
		:>> group = TechnicalProcessGroup::technicalManagement;
		:>> requirementId = "SE-20";
		:>> section = "3.2.15";
	}

	abstract action configurationManagements : ConfigurationManagement[0..*] nonunique :> technicalProcesses;

	action def TechnicalDataManagement :> TechnicalProcess {
		doc /* Technical Data Management process (NPR 7123.1D section 3.2.16, requirement SE-21): The
		 * Technical Data Management Process is used to plan for, acquire, access, manage, protect,
		 * and use data of a technical nature to support the total life cycle of a system. This
		 * process is used to capture trade studies, cost estimates, technical analyses, reports,
		 * and other important information.
		 */
		:>> group = TechnicalProcessGroup::technicalManagement;
		:>> requirementId = "SE-21";
		:>> section = "3.2.16";
	}

	abstract action technicalDataManagements : TechnicalDataManagement[0..*] nonunique :> technicalProcesses;

	action def TechnicalAssessment :> TechnicalProcess {
		doc /* Technical Assessment process (NPR 7123.1D section 3.2.17, requirement SE-22): The
		 * Technical Assessment process is used to help monitor progress of the technical effort and
		 * provide status information for support of the system design, product realization, and
		 * technical management processes. A key aspect of the technical assessment process is the
		 * conduct of life-cycle and technical reviews throughout the system life cycle in
		 * accordance with Chapter 5.
		 */
		:>> group = TechnicalProcessGroup::technicalManagement;
		:>> requirementId = "SE-22";
		:>> section = "3.2.17";
	}

	abstract action technicalAssessments : TechnicalAssessment[0..*] nonunique :> technicalProcesses;

	action def DecisionAnalysis :> TechnicalProcess {
		doc /* Decision Analysis process (NPR 7123.1D section 3.2.18, requirement SE-23): The Decision
		 * Analysis process, including processes for identification of decision criteria,
		 * identification of alternatives, analysis of alternatives, and alternative selection, is
		 * applied to technical issues to support their resolution. It considers relevant data
		 * (e.g., engineering performance, quality, and reliability) and associated uncertainties.
		 * Decision analysis is used throughout the system life cycle to formulate candidate
		 * decision alternatives and evaluate their impacts on health and medical, system security
		 * (including cybersecurity), safety, technical, cost, and schedule performance.
		 * NASA/SP-2010-576, NASA Risk-Informed Decision Making Handbook, provides guidance for
		 * analyzing decision alternatives in a risk-informed fashion.
		 */
		:>> group = TechnicalProcessGroup::technicalManagement;
		:>> requirementId = "SE-23";
		:>> section = "3.2.18";
	}

	abstract action decisionAnalysiss : DecisionAnalysis[0..*] nonunique :> technicalProcesses;

	/* Chapter 5: life cycle, key decision points and reviews */

	enum def LifeCyclePhase {
		doc /* The life-cycle phases named in the header of Table 5-1. NPR 7123.1D does not define life
		 * cycles: 'A specific life cycle may be required by associated project management NPRs. For
		 * example, NPR 7120.5 defines the life cycles for space flight programs and projects, and
		 * NPR 7120.7 ... defines life cycles for IT' (section 5.1.1.2); Figures 5-1 through 5-4
		 * repeat the NPR 7120.5 life cycles for illustration only. The phases are therefore kept
		 * thin here; a program or project takes its life cycle from its governing program/project
		 * management NPR.
		 */
		formulation {
			doc /* The Formulation stage of a program (Table 5-1 header), which closes at PDR. */
		}
		implementation {
			doc /* The Implementation stage of a program (Table 5-1 header), from CDR onward. */
		}
		prePhaseA {
			doc /* Pre-Phase A of a project or single-project program; its life-cycle review is the MCR
			 * (Table 5-1).
			 */
		}
		phaseA {
			doc /* Phase A; its life-cycle reviews are the SRR and the MDR/SDR (Table 5-1). */
		}
		phaseB {
			doc /* Phase B; its life-cycle review is the PDR (Table 5-1). */
		}
		phaseC {
			doc /* Phase C; its life-cycle reviews are the CDR and the SIR (Table 5-1). */
		}
		phaseD {
			doc /* Phase D; its life-cycle reviews are the ORR and the FRR (Table 5-1). */
		}
		phaseE {
			doc /* Phase E; its life-cycle review is the DR (Table 5-1). */
		}
		phaseF {
			doc /* Phase F; its life-cycle review is the DRR (Table 5-1). */
		}
	}

	occurrence def KeyDecisionPoint :> Occurrence {
		doc /* A key decision point (KDP). 'The progress between life-cycle phases is marked by key
		 * decision points (KDPs). At each KDP, management examines the maturity of the technical
		 * aspects of the program/project' (section 5.1.4). Table 5-1 places KDP A through KDP F
		 * after the MCR, MDR/SDR, PDR, SIR, FRR and DR of a project, KDP 0 and KDP I at the SRR and
		 * MDR/SDR of an uncoupled or loosely coupled program and at the MDR/SDR and PDR of a
		 * tightly coupled program, followed by periodic KDPs; the governing program/project
		 * management NPR defines them.
		 */
		attribute designation : String[0..1] {
			doc /* The KDP's designation in its life cycle, e.g. "KDP C". */
		}
		attribute phase : LifeCyclePhase[0..1];
	}

	abstract occurrence keyDecisionPoints : KeyDecisionPoint[0..*] nonunique :> occurrences;

	enum def ReviewKind {
		doc /* Every review of NPR 7123.1D Appendix G, by its acronym (Appendix B). Combined tables
		 * (MDR/SDR, MRR/FRR, PIR/PSR) give one literal per review they cover; the program tables
		 * G-1 and G-2 share the SRR and SDR literals with the project tables, so a program review
		 * states its criteria from the program packages of ReviewCriteria.
		 */
		MCR {
			doc /* Mission Concept Review (Table G-3). */
		}
		SRR {
			doc /* System Requirements Review; Table G-1 for programs, Table G-4 for projects and
			 * single-project programs (Table G-1, G-4).
			 */
		}
		SDR {
			doc /* System Definition Review; Table G-2 for programs, Table G-5 (MDR/SDR) for projects
			 * and single-project programs (Table G-2, G-5).
			 */
		}
		MDR {
			doc /* Mission Definition Review, held with or instead of the SDR by projects and single-
			 * project programs (Table G-5).
			 */
		}
		PDR {
			doc /* Preliminary Design Review (Table G-6). */
		}
		CDR {
			doc /* Critical Design Review (Table G-7). */
		}
		PRR {
			doc /* Production Readiness Review (Table G-8). */
		}
		SIR {
			doc /* System Integration Review (Table G-9). */
		}
		TRR {
			doc /* Test Readiness Review (Table G-10). */
		}
		SAR {
			doc /* System Acceptance Review (Table G-11). */
		}
		ORR {
			doc /* Operational Readiness Review (Table G-12). */
		}
		MRR {
			doc /* Mission Readiness Review (Table G-13). */
		}
		FRR {
			doc /* Flight Readiness Review (Table G-13). */
		}
		PLAR {
			doc /* Post-Launch Assessment Review (Table G-14). */
		}
		CERR {
			doc /* Critical Event Readiness Review (Table G-15). */
		}
		PFAR {
			doc /* Post-Flight Assessment Review (Table G-16). */
		}
		DR {
			doc /* Decommissioning Review (Table G-17). */
		}
		DRR {
			doc /* Disposal Readiness Review (Table G-18). */
		}
		peerReview {
			doc /* Peer Review (Table G-19). */
		}
		PIR {
			doc /* Program Implementation Review (Table G-20). */
		}
		PSR {
			doc /* Program Status Review (Table G-20). */
		}
		DCR {
			doc /* Design Certification Review (Table G-21). */
		}
	}

	enum def ReviewItemKind {
		doc /* The review action documentation of section 6.2.9: 'RIDs, RFAs, and/or Action Items as
		 * established by the project'; section 5.2.3 completes a review once agreement exists on
		 * the disposition of all RIDs and RFAs.
		 */
		requestForAction {
			doc /* A Request for Action (RFA). */
		}
		reviewItemDiscrepancy {
			doc /* A Review Item Discrepancy (RID). */
		}
		actionItem {
			doc /* An action item. */
		}
	}

	item def ReviewItem :> Item {
		doc /* One RFA, RID or action item raised at a review, with the status its burndown tracks
		 * (SE-64 asks for 'a set of review trends ... that includes closure of review action
		 * documentation').
		 */
		attribute kind : ReviewItemKind;
		attribute status : StatusKind[0..1];
		attribute originator : String[0..1];
	}

	abstract item reviewItems : ReviewItem[0..*] nonunique :> items;

	enum def ReviewCriterionKind {
		doc /* Whether a review criterion is an entrance criterion or a success criterion (Appendix G).
		 */
		entrance;
		success;
	}

	abstract requirement def ReviewCriterion :> RequirementCheck {
		doc /* A criterion of a life-cycle or technical review. Appendix G 'provides guidance on
		 * entrance and success criteria' and says of its tables: 'The technical team should utilize
		 * the guidance defined in Appendix G as well as Center best practices for defining entrance
		 * and success criteria' (section 5.2.2.1). Section G.1.2 expects tailoring: programs and
		 * projects tailor the reviews and customize the criteria to their size, complexity and
		 * unique needs (section 5.1.3). A project states a review's criteria by subsetting the
		 * transcribed definitions of ReviewCriteria into the review's `entranceCriteria` and
		 * `successCriteria`, redefining the text of the ones it customizes and adding its own.
		 */
		attribute kind : ReviewCriterionKind;
		attribute number : Integer[0..1] {
			doc /* The number of the criterion in its Appendix G table. */
		}
	}

	requirement def EntranceCriterion :> ReviewCriterion {
		doc /* 'Entrance Criteria: Guidance for minimum accomplishments the program or project fulfills
		 * prior to a life-cycle review.' (Appendix A)
		 */
		:>> kind = ReviewCriterionKind::entrance;
	}

	abstract requirement reviewEntranceCriteria : EntranceCriterion[0..*] nonunique :> requirementChecks;

	requirement def SuccessCriterion :> ReviewCriterion {
		doc /* 'Success Criteria: Specific accomplishments that need to be satisfactorily demonstrated
		 * to meet the objectives of a life-cycle and technical review so that a technical effort
		 * can progress further in the life cycle. Success criteria are documented in the
		 * corresponding technical review plan.' (Appendix A)
		 */
		:>> kind = ReviewCriterionKind::success;
	}

	abstract requirement reviewSuccessCriteria : SuccessCriterion[0..*] nonunique :> requirementChecks;

	abstract action def TechnicalReview :> Action {
		doc /* 'A technical review is an evaluation of the program/project, or element thereof, by the
		 * technical team and other knowledgeable participants for the purposes of: a. Assessing the
		 * status of and progress toward accomplishing the planned activities. b. Validating the
		 * technical tradeoffs explored and design solutions proposed. c. Identifying technical
		 * weaknesses or marginal design and potential problems (risks) and recommending
		 * improvements and corrective actions. d. Making judgments on the activity's readiness for
		 * the follow-on events ... e. Making assessments and recommendations to the program/project
		 * team, Center, and Agency management. f. Providing a historical record of decisions ... g.
		 * Assessing the technical risk status and current risk profile.' (section 5.2.2.9).
		 * 'Technical teams shall monitor technical effort through periodic technical reviews
		 * [SE-57].' A review states its entrance and success criteria, the work products it reviews
		 * and the RFAs, RIDs and action items it raises.
		 */
		requirement entranceCriteria : EntranceCriterion[0..*] :> reviewEntranceCriteria;
		requirement successCriteria : SuccessCriterion[0..*] :> reviewSuccessCriteria;
		item reviewedProducts : WorkProduct[0..*] :> workProducts;
		item items : ReviewItem[0..*] :> reviewItems;
		attribute status : StatusKind[0..1];
	}

	abstract action technicalReviews : TechnicalReview[0..*] nonunique :> actions;

	action def LifeCycleReview :> TechnicalReview {
		doc /* A life-cycle review of chapter 5: 'Each program and project will perform the life-cycle
		 * reviews as required by or tailored in accordance with their governing program/project
		 * management NPR, applicable Center policies and procedures, and the requirements of this
		 * document. These reviews provide a periodic assessment of a program or project's technical
		 * and programmatic status and health at key points in the life cycle.' (section 5.1.3).
		 * 'The technical team shall participate in the life-cycle and technical reviews as
		 * indicated in the governing program/project management NPR [SE-33].' A life-cycle review
		 * states its kind, the minimum products of section 5.2.2.2 it reviews at their required
		 * maturity, and its criteria; it is complete when the conditions of section 5.2.3.1 are
		 * met.
		 */
		attribute kind : ReviewKind;
		attribute phase : LifeCyclePhase[0..1];
		ref occurrence keyDecisionPoint : KeyDecisionPoint[0..1] :> keyDecisionPoints {
			doc /* The KDP the review informs, if the life cycle places one after it. */
		}
	}

	abstract action lifeCycleReviews : LifeCycleReview[0..*] nonunique :> technicalReviews;

	action def PeerReview :> TechnicalReview {
		doc /* A peer review (Table G-19): peer reviews 'are focused, in-depth technical reviews that
		 * support the evolving design and development of a product, including critical
		 * documentation or data packages. The participants in a peer review are the technical
		 * experts and key stakeholders for the product being reviewed. Peer reviews ... are planned
		 * as part of the program/project review plan and are described in the SEMP.' Section 5.1.5
		 * recommends them 'to reduce risk through expert knowledge infusion, confirmation of
		 * approach, and identification of defects'.
		 */
	}

	abstract action peerReviews : PeerReview[0..*] nonunique :> technicalReviews;

	/* Section 5.2.2.2, Table 5-1 and Appendix F: work products and their maturity */

	enum def WorkProductMaturity {
		doc /* The technical work product maturity terminology of Appendix F. 'F.1 For non-
		 * configuration-controlled documents, the following terms and definitions are used in this
		 * document: Initial, Final, Update. F.2 For configuration-controlled documents ...:
		 * Preliminary, Baseline, Approve, Update.' Update is one literal; Appendix F gives it the
		 * same meaning in both vocabularies. Table 5-1 also marks one cell 'Approach', which
		 * Appendix F does not define.
		 */
		initial {
			doc /* 'Initial is applied to products that are continually developed and updated as the
			 * program or project matures.' (F.1.a)
			 */
		}
		final {
			doc /* 'Final is applied to products that are expected to exist in this final form, e.g.,
			 * minutes and final reports.' (F.1.b)
			 */
		}
		update {
			doc /* 'Update is applied to products that are expected to evolve as the formulation and
			 * implementation processes evolve. Only expected updates are indicated. However, any
			 * document may be updated as needed.' (F.1.c, F.2.d) 'Updates to baselined documents
			 * require the same formal approval process as the original baseline.' (F.2.d)
			 */
		}
		preliminary {
			doc /* 'Preliminary is the documentation of information as it stabilizes but before it goes
			 * under configuration control. It is the initial development leading to a baseline.
			 * Some products will remain in a preliminary state for multiple reviews. The initial
			 * preliminary version is likely to be updated at a subsequent review but remains
			 * preliminary until baselined.' (F.2.a)
			 */
		}
		baseline {
			doc /* 'Baseline indicates putting the product under configuration control so that changes
			 * can be tracked, approved, and communicated to the team and any relevant stakeholders.
			 * The expectation on products labeled baseline is that they will be at least final
			 * drafts going into the designated review and baselined coming out of the review.
			 * Baselining a product does not necessarily imply that it is fully mature at that point
			 * in the life cycle.' (F.2.b)
			 */
		}
		approve {
			doc /* 'Approve is used for a product, such as Concept Documentation, that is not expected
			 * to be put under classic configuration control but still requires that changes from
			 * the Approved version are documented at each subsequent Update.' (F.2.c)
			 */
		}
		approach {
			doc /* The maturity Table 5-1 states for the Verification and Validation Plan at MCR;
			 * Appendix F does not define the term, and the library reads it as the plan's approach
			 * being stated before the preliminary plan exists.
			 */
		}
	}

	metadata def MaturityExpectation {
		doc /* One cell of Table 5-1 'SE Work Product Maturity': the maturity a work product is expected
		 * to have at a life-cycle review. `required` is true where Table 5-1 marks the cell with
		 * ('Item is a required product for that review'), which are exactly the minimum products of
		 * section 5.2.2.2; `requirementId` then names the [SE-nn] requirement that asks for it. The
		 * library's work product definitions carry one annotation per cell, so a tool can read the
		 * whole table from the model.
		 */
		attribute review : ReviewKind;
		attribute maturity : WorkProductMaturity;
		attribute required : Boolean default false;
		attribute requirementId : String[0..1];
		attribute note : String[0..1];
	}

	item def WorkProduct :> Item {
		doc /* A technical work product: 'The application of the common technical processes within each
		 * life-cycle phase produces technical results and work products that provide inputs to
		 * life-cycle and technical reviews and support informed management decisions for
		 * progressing to the next life-cycle phase.' (section 5.1.2). A usage states the maturity
		 * the product has reached; the definitions below record, as MaturityExpectation
		 * annotations, the maturity Table 5-1 expects of each product at each review. 'The
		 * expectation for work products identified as baselined in Section 5.2.2.2 and Table 5-1 is
		 * that they will be at least final drafts going into the designated life-cycle review.'
		 * (section 5.2.2.4)
		 */
		attribute maturity : WorkProductMaturity[0..1];
	}

	abstract item workProducts : WorkProduct[0..*] nonunique :> items;

	item def StakeholderIdentificationAndExpectationsDefinition :> WorkProduct {
		doc /* The stakeholder identification and expectation definitions (section 5.2.2.2.a(1), SE-35),
		 * the output of the Stakeholder Expectations Definition process.
		 */
		@MaturityExpectation { review = ReviewKind::MCR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-35"; }
		@MaturityExpectation { review = ReviewKind::SRR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::update; }
	}

	abstract item stakeholderIdentificationAndExpectationsDefinitions : StakeholderIdentificationAndExpectationsDefinition[0..*] nonunique :> workProducts;

	item def ConceptDefinition :> WorkProduct {
		doc /* The concept definition (section 5.2.2.2.a(2), SE-36). */
		@MaturityExpectation { review = ReviewKind::MCR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-36"; }
		@MaturityExpectation { review = ReviewKind::SRR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
	}

	abstract item conceptDefinitions : ConceptDefinition[0..*] nonunique :> workProducts;

	item def MeasureOfEffectivenessDefinition :> WorkProduct {
		doc /* The Measures of Effectiveness (MOE) definition (section 5.2.2.2.a(3), SE-37). State the
		 * measures themselves with the standard `#moe` keyword of ParametersOfInterestMetadata.
		 */
		@MaturityExpectation { review = ReviewKind::MCR; maturity = WorkProductMaturity::approve; required = true; requirementId = "SE-37"; }
	}

	abstract item measureOfEffectivenessDefinitions : MeasureOfEffectivenessDefinition[0..*] nonunique :> workProducts;

	item def CostAndScheduleForTechnicalImplementation :> WorkProduct {
		doc /* The cost and schedule for technical implementation (Table 5-1). */
		@MaturityExpectation { review = ReviewKind::MCR; maturity = WorkProductMaturity::initial; }
		@MaturityExpectation { review = ReviewKind::SRR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::baseline; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::ORR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::FRR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::DR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::DRR; maturity = WorkProductMaturity::update; }
	}

	abstract item costAndScheduleForTechnicalImplementations : CostAndScheduleForTechnicalImplementation[0..*] nonunique :> workProducts;

	item def RequirementSet :> WorkProduct {
		doc /* The requirements of the product layer, the Requirements row of Table 5-1 (section
		 * 5.2.2.2.b(2), SE-39).
		 */
		@MaturityExpectation { review = ReviewKind::MCR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::SRR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-39"; }
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
	}

	abstract item requirementSets : RequirementSet[0..*] nonunique :> workProducts;

	item def TechnicalPerformanceMeasureDefinition :> WorkProduct {
		doc /* The Technical Performance Measures (TPM) definition (section 5.2.2.2.c(1), SE-40;
		 * sections 6.2.6 to 6.2.8, SE-60 to SE-63). State the measures themselves with the standard
		 * `#mop` keyword of ParametersOfInterestMetadata, which has no separate keyword for a TPM.
		 */
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::approve; required = true; requirementId = "SE-40"; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::approve; required = true; requirementId = "SE-40"; }
	}

	abstract item technicalPerformanceMeasureDefinitions : TechnicalPerformanceMeasureDefinition[0..*] nonunique :> workProducts;

	item def HumanSystemsIntegrationApproach :> WorkProduct {
		doc /* The Human Systems Integration (HSI) approach (section 5.2.1.3, SE-65; section
		 * 5.2.2.2.b(3), SE-66).
		 */
		@MaturityExpectation { review = ReviewKind::MCR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::SRR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-66"; }
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
	}

	abstract item humanSystemsIntegrationApproachs : HumanSystemsIntegrationApproach[0..*] nonunique :> workProducts;

	item def ArchitectureDefinition :> WorkProduct {
		doc /* The architecture definition (section 5.2.2.2.c(2), SE-41). */
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-41"; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-41"; }
	}

	abstract item architectureDefinitions : ArchitectureDefinition[0..*] nonunique :> workProducts;

	item def AllocationOfRequirementsToNextLowerLevel :> WorkProduct {
		doc /* The allocation of requirements to the next lower level (section 5.2.2.2.c(3), SE-42).
		 */
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-42"; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-42"; }
	}

	abstract item allocationOfRequirementsToNextLowerLevels : AllocationOfRequirementsToNextLowerLevel[0..*] nonunique :> workProducts;

	item def RequiredLeadingIndicatorTrends :> WorkProduct {
		doc /* The trend of the required leading indicators (section 5.2.2.2.c(4), SE-43): mass margins
		 * (SE-62), power margins (SE-63) and review trends (SE-64). State the indicators themselves
		 * with the standard `#mop` keyword of ParametersOfInterestMetadata.
		 */
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::initial; required = true; requirementId = "SE-43"; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::initial; required = true; requirementId = "SE-43"; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::update; }
	}

	abstract item requiredLeadingIndicatorTrendss : RequiredLeadingIndicatorTrends[0..*] nonunique :> workProducts;

	item def DesignSolution :> WorkProduct {
		doc /* The design solution definition: preliminary at PDR (section 5.2.2.2.d(1), SE-45) and the
		 * baseline detailed design at CDR (section 5.2.2.2.e, SE-46).
		 */
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::preliminary; required = true; requirementId = "SE-45"; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-46"; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::ORR; maturity = WorkProductMaturity::update; }
	}

	abstract item designSolutions : DesignSolution[0..*] nonunique :> workProducts;

	item def InterfaceDefinitionProduct :> WorkProduct {
		doc /* The interface definition(s) (Table 5-1), the output of the Interface Management process.
		 */
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::baseline; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::update; }
	}

	abstract item interfaceDefinitionProducts : InterfaceDefinitionProduct[0..*] nonunique :> workProducts;

	item def ImplementationPlan :> WorkProduct {
		doc /* The implementation plan(s) (make/code, buy, reuse) (Table 5-1). */
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::baseline; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
	}

	abstract item implementationPlans : ImplementationPlan[0..*] nonunique :> workProducts;

	item def IntegrationPlan :> WorkProduct {
		doc /* The integration plan(s) (section 5.2.2.2.d(2), SE-67; section 5.2.2.2.f(1), SE-47).
		 */
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-67"; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::update; required = true; requirementId = "SE-47"; }
	}

	abstract item integrationPlans : IntegrationPlan[0..*] nonunique :> workProducts;

	item def VerificationAndValidationPlan :> WorkProduct {
		doc /* The Verification and Validation Plan (section 5.2.2.2.d(3), SE-68). */
		@MaturityExpectation { review = ReviewKind::MCR; maturity = WorkProductMaturity::approach; }
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-68"; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::update; }
	}

	abstract item verificationAndValidationPlans : VerificationAndValidationPlan[0..*] nonunique :> workProducts;

	item def VerificationAndValidationResults :> WorkProduct {
		doc /* The Verification and Validation (V&V) results (section 5.2.2.2.f(2), SE-48; g(1), SE-69;
		 * h(1), SE-53).
		 */
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::initial; required = true; requirementId = "SE-48"; }
		@MaturityExpectation { review = ReviewKind::ORR; maturity = WorkProductMaturity::preliminary; required = true; requirementId = "SE-69"; }
		@MaturityExpectation { review = ReviewKind::FRR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-53"; }
	}

	abstract item verificationAndValidationResultss : VerificationAndValidationResults[0..*] nonunique :> workProducts;

	item def TransportationCriteriaAndInstructions :> WorkProduct {
		doc /* The transportation criteria and instructions (Table 5-1). */
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::initial; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::final; }
		@MaturityExpectation { review = ReviewKind::ORR; maturity = WorkProductMaturity::update; }
	}

	abstract item transportationCriteriaAndInstructionss : TransportationCriteriaAndInstructions[0..*] nonunique :> workProducts;

	item def CertificationForFlightOrUse :> WorkProduct {
		doc /* The certification for flight/use (section 5.2.2.2.h(2), SE-54). */
		@MaturityExpectation { review = ReviewKind::ORR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::FRR; maturity = WorkProductMaturity::final; required = true; requirementId = "SE-54"; }
	}

	abstract item certificationForFlightOrUses : CertificationForFlightOrUse[0..*] nonunique :> workProducts;

	item def DecommissioningPlan :> WorkProduct {
		doc /* The decommissioning plan(s) (section 5.2.2.2.g(2), SE-51; section 5.2.2.2.i, SE-55).
		 */
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::ORR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-51"; }
		@MaturityExpectation { review = ReviewKind::FRR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::DR; maturity = WorkProductMaturity::update; required = true; requirementId = "SE-55"; }
	}

	abstract item decommissioningPlans : DecommissioningPlan[0..*] nonunique :> workProducts;

	item def DisposalPlan :> WorkProduct {
		doc /* The disposal plan(s) (section 5.2.2.2.g(3), SE-52; section 5.2.2.2.j, SE-56). */
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::ORR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-52"; }
		@MaturityExpectation { review = ReviewKind::FRR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::DR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::DRR; maturity = WorkProductMaturity::update; required = true; requirementId = "SE-56"; }
	}

	abstract item disposalPlans : DisposalPlan[0..*] nonunique :> workProducts;

	/* Chapter 6: the Systems Engineering Management Plan */

	part def SEMP :> Document, WorkProduct {
		doc /* The Systems Engineering Management Plan. 'The SEMP identifies the roles and
		 * responsibility interfaces of the technical effort and how those interfaces will be
		 * managed. The SEMP is the vehicle that documents and communicates the technical approach,
		 * including the application of the common technical processes; resources to be used; and
		 * key technical tasks, activities, and events along with their metrics and success
		 * criteria.' (Appendix A). 'The technical teams shall define in the program/project SEMP
		 * how the required 17 common technical processes, as tailored, will be recursively applied
		 * to the various levels of program/project product layer system structure during each
		 * applicable life-cycle phase [SE-58].' A project specialises SEMP and nests one usage of
		 * each of the 17 process definitions, with its `tailoring`, under it; approved tailoring of
		 * reviews and criteria is captured there too (section 5.1.3). Table 5-1 expects the SEMP
		 * preliminary at MCR, baselined at SRR for projects, single-project programs and one-step
		 * AO programs (SE-38) or at MDR/SDR for uncoupled, loosely coupled, tightly coupled and
		 * two-step AO programs (SE-44), and updated at PDR, CDR and SIR. 'The ETA shall approve the
		 * SEMP, waiver or deviation authorizations, and other key technical documents to ensure
		 * independent assessment of technical content [SE-06].'
		 */
		:>> title default = "Systems Engineering Management Plan";
		@MaturityExpectation { review = ReviewKind::MCR; maturity = WorkProductMaturity::preliminary; }
		@MaturityExpectation { review = ReviewKind::SRR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-38"; note = "For projects, single-project programs, and a one-step AO programs."; }
		@MaturityExpectation { review = ReviewKind::MDR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-44"; note = "For uncoupled, tightly coupled, loosely coupled, and two-step AO programs."; }
		@MaturityExpectation { review = ReviewKind::SDR; maturity = WorkProductMaturity::baseline; required = true; requirementId = "SE-44"; note = "For uncoupled, tightly coupled, loosely coupled, and two-step AO programs."; }
		@MaturityExpectation { review = ReviewKind::PDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::CDR; maturity = WorkProductMaturity::update; }
		@MaturityExpectation { review = ReviewKind::SIR; maturity = WorkProductMaturity::update; }
	}

	abstract part semps : SEMP[0..*] nonunique :> parts, workProducts;

	/* Appendix E: technology readiness levels */

	attribute def TechnologyReadinessLevelValue {
		doc /* The level number, definition and success criteria an Appendix E technology readiness
		 * level carries.
		 */
		attribute level : Integer;
		attribute definition : String;
		attribute successCriteria : String;
	}

	enum def TechnologyReadinessLevel :> TechnologyReadinessLevelValue {
		doc /* 'Technology Readiness Level: A scale against which to measure the maturity of a
		 * technology. TRLs range from 1 (Basic Technology Research) to 9 (Systems Test, Launch, and
		 * Operations).' (Appendix A). Each literal carries the definition and the success criteria
		 * of Appendix E; the hardware and software descriptions and the examples of Appendix E are
		 * not transcribed.
		 */
		trl1 {
			doc /* TRL 1: Basic principles observed and reported. */
			:>> level = 1;
			:>> definition = "Basic principles observed and reported.";
			:>> successCriteria = "Peer reviewed documentation of research underlying the proposed concept/application.";
		}
		trl2 {
			doc /* TRL 2: Technology concept and/or application formulated. */
			:>> level = 2;
			:>> definition = "Technology concept and/or application formulated.";
			:>> successCriteria = "Documented description of the application/concept that addresses feasibility and benefit.";
		}
		trl3 {
			doc /* TRL 3: Analytical and experimental proof-of-concept of critical function and/or
			 * characteristics.
			 */
			:>> level = 3;
			:>> definition = "Analytical and experimental proof-of-concept of critical function and/or characteristics.";
			:>> successCriteria = "Documented analytical/experimental results validating predictions of key parameters.";
		}
		trl4 {
			doc /* TRL 4: Component and/or breadboard validation in a laboratory environment. */
			:>> level = 4;
			:>> definition = "Component and/or breadboard validation in a laboratory environment.";
			:>> successCriteria = "Documented test performance demonstrating agreement with analytical predictions. Documented definition of potentially relevant environment.";
		}
		trl5 {
			doc /* TRL 5: Component and/or brassboard validated in a relevant environment. */
			:>> level = 5;
			:>> definition = "Component and/or brassboard validated in a relevant environment.";
			:>> successCriteria = "Documented test performance demonstrating agreement with analytical predictions. Documented definition of scaling requirements. Performance predictions are made for subsequent development phases.";
		}
		trl6 {
			doc /* TRL 6: System/sub-system model or prototype demonstration in a relevant environment.
			 */
			:>> level = 6;
			:>> definition = "System/sub-system model or prototype demonstration in a relevant environment.";
			:>> successCriteria = "Documented test performance demonstrating agreement with analytical predictions.";
		}
		trl7 {
			doc /* TRL 7: System prototype demonstration in an operational environment. */
			:>> level = 7;
			:>> definition = "System prototype demonstration in an operational environment.";
			:>> successCriteria = "Documented test performance demonstrating agreement with analytical predictions.";
		}
		trl8 {
			doc /* TRL 8: Actual system completed and "flight qualified" through test and demonstration.
			 */
			:>> level = 8;
			:>> definition = "Actual system completed and 'flight qualified' through test and demonstration.";
			:>> successCriteria = "Documented test performance verifying analytical predictions.";
		}
		trl9 {
			doc /* TRL 9: Actual system flight proven through successful mission operations. */
			:>> level = 9;
			:>> definition = "Actual system flight proven through successful mission operations.";
			:>> successCriteria = "Documented mission operational results.";
		}
	}

	metadata def <technologyReadiness> TechnologyReadiness {
		doc /* Records the technology readiness level assessed for the annotated element and, if any,
		 * the review at which it was assessed: `part lidar { @TechnologyReadiness { level =
		 * TechnologyReadinessLevel::trl6; } }`.
		 */
		attribute level : TechnologyReadinessLevel;
		attribute assessedAt : ReviewKind[0..1];
		attribute basis : String[0..1];
	}

	/* Chapter 2: Technical Authority, tailoring, waivers and deviations */

	enum def TechnicalAuthorityKind {
		doc /* The three lines of Technical Authority named in Appendix A: 'Engineering, Safety and
		 * Mission Assurance, and Health and Medical'.
		 */
		engineering {
			doc /* The Engineering Technical Authority (ETA): 'ETA includes individuals who have been
			 * formally delegated Technical Authority that flows from the Administrator to the NASA
			 * Chief Engineer and to the Center Directors for further delegation to Center
			 * engineering leadership and individuals.' (Appendix A)
			 */
		}
		safetyAndMissionAssurance;
		healthAndMedical;
	}

	part def TechnicalAuthority :> Part {
		doc /* 'Technical Authority: Part of NASA's system of checks and balances that provides
		 * independent oversight of programs and projects in support of safety and mission success
		 * through the selection of individuals at delegated levels of authority. These individuals
		 * are the Technical Authorities. Technical Authority delegations are formal and traceable
		 * to the Administrator.' (Appendix A)
		 */
		attribute kind : TechnicalAuthorityKind;
		attribute holder : String[0..1];
	}

	abstract part technicalAuthorities : TechnicalAuthority[0..*] nonunique :> parts;

	metadata def <waiver> Waiver {
		doc /* 'Waiver: A documented authorization releasing a program or project from meeting a
		 * requirement after the requirement is put under configuration control at the level the
		 * requirement will be implemented.' (Appendix A). Tailoring 'results in deviations or
		 * waivers to requirements depending on the timing of the request' (section 2.2.1.2); the
		 * ETA approves waiver authorizations (SE-06).
		 */
		attribute requirementId : String[0..1];
		attribute rationale : String[0..1];
		attribute approvedBy : String[0..1];
	}

	metadata def <deviation> Deviation {
		doc /* 'Deviation: A documented authorization releasing a program or project from meeting a
		 * requirement before the requirement is put under configuration control at the level the
		 * requirement will be implemented.' (Appendix A). The ETA approves deviation authorizations
		 * (SE-06).
		 */
		attribute requirementId : String[0..1];
		attribute rationale : String[0..1];
		attribute approvedBy : String[0..1];
	}

	/* Appendix H: the active requirements and the compliance matrix */

	requirement def SERequirement :> RequirementCheck {
		doc /* One [SE-nn] requirement of NPR 7123.1D. The documentation of each definition in
		 * SERequirements is the requirement statement; `id`, `section` and `rationale` are the Req
		 * ID, NPR Section and Rationale columns of Table H-1. A project states its compliance with
		 * each by annotating a usage of the definition with `@Compliance`.
		 */
		attribute id : String;
		attribute section : String;
		attribute rationale : String;
	}

	abstract requirement seRequirements : SERequirement[0..*] nonunique :> requirementChecks;

	package SERequirements {
		doc /* The 62 requirements of NPR 7123.1D that are active in revision D, one definition each,
		 * named by their Req ID. SE-01 through SE-05, SE-49 and SE-50 were deleted (Appendix J,
		 * Table J-1: 'Rather than resequence the remaining requirements, the original requirement
		 * numbering was left intact in case Centers or other organizations refer to these
		 * requirement numbers') and are not modelled. Table H-1 lists SE-59 through SE-64 in one
		 * row; the library gives each its statement from sections 6.2.5 to 6.2.9 and the part of
		 * the shared rationale that speaks to it.
		 */

		requirement def <'SE-06'> SE06 :> SERequirement {
			doc /* The ETA shall approve the SEMP, waiver or deviation authorizations, and other key
			 * technical documents to ensure independent assessment of technical content.
			 */
			:>> id = "SE-06";
			:>> section = "6.1.5";
			:>> rationale = "This requirement ensures that the ETA has reviewed and approved of key systems engineering documents.";
		}

		requirement def <'SE-07'> SE07 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Stakeholder
			 * Expectations Definition process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for the definition of stakeholder
			 * expectations for the applicable product layer.
			 */
			:>> id = "SE-07";
			:>> section = "3.2.2.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will gather and address stakeholder expectations. This ensures that the program/project will gain a thorough understanding of what the customer and other stakeholders expect.";
		}

		requirement def <'SE-08'> SE08 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Technical
			 * Requirements Definition process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for the definition of technical
			 * requirements from the set of agreed upon stakeholder expectations for the applicable
			 * product layer.
			 */
			:>> id = "SE-08";
			:>> section = "3.2.3.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will select and gain agreement on the technical requirements.";
		}

		requirement def <'SE-09'> SE09 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Logical
			 * Decomposition process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for logical decomposition of the validated
			 * technical requirements of the applicable product layer.
			 */
			:>> id = "SE-09";
			:>> section = "3.2.4.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will take the technical requirements for the program/project and glean from them what is needed to accomplish them (e.g., functional block diagrams, timing, architectures). This places the requirements into context and ensures they are understood well enough to begin the design process.";
		}

		requirement def <'SE-10'> SE10 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Design Solution
			 * Definition process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for designing product solution definitions
			 * within the applicable product layer that satisfy the derived technical requirements.
			 */
			:>> id = "SE-10";
			:>> section = "3.2.5.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will take the information from the stakeholder expectations, requirements, and logical decomposition and perform the design function. Since all designs are unique, this will describe the general steps that are taken. The specifics for each of the program/projects will be documented in the SEMP or other equivalent program/project documentation.";
		}

		requirement def <'SE-11'> SE11 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Product
			 * Implementation process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for implementation of a design solution
			 * definition by making, buying, or reusing an end product of the applicable product
			 * layer.
			 */
			:>> id = "SE-11";
			:>> section = "3.2.6.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will execute the designs, whether through buying items off the shelf or contracting to have them built, building/coding them within the Center, or reusing products already developed by another program/project. The specifics for how each program/project will make this determination for the various components/assemblies within the product hierarchy are documented in the SEMP or other equivalent program/project documentation.";
		}

		requirement def <'SE-12'> SE12 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Product
			 * Integration process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for the integration of lower level products
			 * into an end product of the applicable product layer in accordance with its design
			 * solution definition.
			 */
			:>> id = "SE-12";
			:>> section = "3.2.7.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will approach the integration of products within successive levels of the product hierarchy. This ensures that planning is performed that will enable a smooth integration of products into higher level assemblies.";
		}

		requirement def <'SE-13'> SE13 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Product
			 * Verification process to include activities, requirements/specifications, guidelines,
			 * and documentation, as tailored and customized for verification of end products
			 * generated by the product implementation process or product integration process
			 * against their design solution definitions.
			 */
			:>> id = "SE-13";
			:>> section = "3.2.8.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will verify that the end products will comply with each of the technical requirements.";
		}

		requirement def <'SE-14'> SE14 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Product
			 * Validation process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for validation of end products generated by
			 * the product implementation process or product integration process against their
			 * stakeholder expectations.
			 */
			:>> id = "SE-14";
			:>> section = "3.2.9.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will show that the end products will meet the stakeholder expectations in the intended environment. This is in addition to verifying they meet the stated requirements and ensures the stakeholder is getting what was expected.";
		}

		requirement def <'SE-15'> SE15 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Product
			 * Transition process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for transitioning end products to the next
			 * higher level product layer customer or user.
			 */
			:>> id = "SE-15";
			:>> section = "3.2.10.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will handle the end products as they move from one location to another. This includes shipping, handling, transportation criteria, physical security, system security (including cybersecurity), and receiving facility storage needs. It ensures that receiving facilities are ready to accept the product and that no damage occurs to the product during handling and transportation.";
		}

		requirement def <'SE-16'> SE16 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Technical
			 * Planning process to include activities, requirements, guidelines, and documentation,
			 * as tailored and customized for planning the technical effort.
			 */
			:>> id = "SE-16";
			:>> section = "3.2.11.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will perform and document all the technical planning for the program/project. This includes all plans developed for the technical effort Systems Engineering Management Plans, risk plans, integration plans, and V&V plans. This ensures that the program/project teams are thinking ahead for the work to be performed and capturing that information so it can be communicated to the rest of the team, customers, and other stakeholders.";
		}

		requirement def <'SE-17'> SE17 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Requirements
			 * Management process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for management of requirements throughout
			 * the system life cycle.
			 */
			:>> id = "SE-17";
			:>> section = "3.2.12.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will handle tracking and changes to the baselined set of requirements. It defines who has authority to submit and approve changes and how requirements are tracked as they flow down to other elements in the product breakdown structure. This ensures that changes to requirements are evaluated and that their impacts are understood and communicated to the rest of the team.";
		}

		requirement def <'SE-18'> SE18 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Interface
			 * Management process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for management of the interfaces defined
			 * and generated during the application of the system design processes.
			 */
			:>> id = "SE-18";
			:>> section = "3.2.13.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will manage the internal and external interfaces of their end product. This will ensure compatibility when the various parts of the system are brought together for assembly/integration.";
		}

		requirement def <'SE-19'> SE19 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Technical Risk
			 * Management process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for management of the risk identified
			 * during the technical effort.
			 */
			:>> id = "SE-19";
			:>> section = "3.2.14.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will handle the technical portions of the program/project risks and report them for inclusion with the cost and schedule risk portions. It ensures that the technical aspects of risks to the program/projects successful execution are captured and reported to program/project management who will be developing the overall risk posture.";
		}

		requirement def <'SE-20'> SE20 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Configuration
			 * Management process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for configuration management.
			 */
			:>> id = "SE-20";
			:>> section = "3.2.15.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will perform configuration management of the end products, enabling products and other work products key to the program/project. The technical products to be controlled are identified and tracked to ensure that the team knows what the configuration of their system is at all phases of the life cycle.";
		}

		requirement def <'SE-21'> SE21 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Technical Data
			 * Management process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for management of the technical data
			 * generated and used in the technical effort.
			 */
			:>> id = "SE-21";
			:>> section = "3.2.16.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will handle all the technical data that is generated by the program/project. This will include all data needed to manage, operate, and support the system products over the life cycle. It ensures that the data is available and secure when needed.";
		}

		requirement def <'SE-22'> SE22 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Technical
			 * Assessment process to include activities, requirements, guidelines, and
			 * documentation, as tailored and customized for making assessments of the progress of
			 * planned technical effort and progress toward requirements satisfaction.
			 */
			:>> id = "SE-22";
			:>> section = "3.2.17.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identifies how they will assess the progress of the program/project's technical efforts, including life-cycle reviews. It ensures that the program/project team, customers, and other key stakeholders know how the effort is progressing and if additional actions are needed to resolve issues prior to becoming too costly.";
		}

		requirement def <'SE-23'> SE23 :> SERequirement {
			doc /* Program/Project Managers shall identify and implement an ETA-approved Decision
			 * Analysis process to include activities, requirements, guidelines, and documentation,
			 * as tailored and customized for making technical decisions.
			 */
			:>> id = "SE-23";
			:>> section = "3.2.18.1";
			:>> rationale = "This requirement ensures that the program/project and the ETA identify how they will make and document key technical decisions. It helps to ensure that all team members know who can make decisions, what their authority levels are, and where to go to gain an understanding of what key decisions have been made.";
		}

		requirement def <'SE-24'> SE24 :> SERequirement {
			doc /* The NASA technical team shall define the engineering activities for the periods
			 * before contract award, during contract performance, and upon contract completion in
			 * the SEMP or other equivalent program/project documentation.
			 */
			:>> id = "SE-24";
			:>> section = "4.2.1";
			:>> rationale = "It is important for both the Government and contractor technical teams to understand what activities will be handled by which organization throughout the product life cycle. The contractor(s) will typically develop a SEMP or other equivalent program/project documentation to describe the technical activities in their portion of the project, but an overarching SEMP (or other equivalent program/project documentation) is needed that will describe all technical activities across the life cycle whether contracted or not.";
		}

		requirement def <'SE-25'> SE25 :> SERequirement {
			doc /* The NASA technical team shall establish the technical inputs to the solicitation
			 * appropriate for the product(s) to be developed, including product requirements and
			 * Statement of Work tasks.
			 */
			:>> id = "SE-25";
			:>> section = "4.2.3";
			:>> rationale = "The technical team's participation in the development of the solicitation is critical to enabling a successful contracted effort. Ensuring that the proper application of the common technical processes into the contracted effort will enhance the chances for success.";
		}

		requirement def <'SE-26'> SE26 :> SERequirement {
			doc /* The NASA technical team shall determine the technical work products to be delivered
			 * by the offeror or contractor, to include contractor documentation that specifies the
			 * contractor's SE approach to the scope of activities described by the 17 common
			 * technical processes.
			 */
			:>> id = "SE-26";
			:>> section = "4.2.4";
			:>> rationale = "The technical team is in the best position to determine what kind of work products from the technical effort will need to be delivered. These products will eventually be used by the technical team to determine the suitability of the contracted effort in its ability to meet requirements, satisfy the stakeholder expectations, and perform as planned.";
		}

		requirement def <'SE-27'> SE27 :> SERequirement {
			doc /* The NASA technical team shall provide the requirements for technical insight and
			 * oversight activities planned in the NASA SEMP or other equivalent program/project
			 * documentation to the contracting officer for inclusion in the solicitation.
			 */
			:>> id = "SE-27";
			:>> section = "4.2.5";
			:>> rationale = "In addition to the work description and products to be delivered, how the technical team will gain an adequate understanding of the contracted work, what authority (if any) they will have to direct or influence the work, and their participation at key life-cycle reviews. In the end the technical team needs enough information to advise the Program/Project Manager and ETA as to the adequacy of the technical work.";
		}

		requirement def <'SE-28'> SE28 :> SERequirement {
			doc /* The NASA technical team shall participate in the evaluation of offeror proposals in
			 * accordance with applicable NASA and Center source selection procedures.
			 */
			:>> id = "SE-28";
			:>> section = "4.2.7";
			:>> rationale = "Technical personnel will need to be involved in reviewing the proposals and providing advice/guidance on their merits. These personnel may or may not be part of the technical team that will execute the program/project.";
		}

		requirement def <'SE-29'> SE29 :> SERequirement {
			doc /* The NASA technical team, under the authority of the contracting officer, shall
			 * perform the technical insight and oversight activities established in the contract
			 * including modifications to the original contract.
			 */
			:>> id = "SE-29";
			:>> section = "4.3.1";
			:>> rationale = "After the contract is awarded, the contracting officer will depend on the technical team to execute the oversight/ insight of the technical work as defined in their SEMP (or other equivalent program/project documentation) and the contract.";
		}

		requirement def <'SE-30'> SE30 :> SERequirement {
			doc /* The NASA technical team shall participate in the review(s) to finalize Government
			 * acceptance of the deliverables.
			 */
			:>> id = "SE-30";
			:>> section = "4.4.1";
			:>> rationale = "Per the agreement in the SEMP (or other equivalent program/project documentation) and the contract, the technical team will participate in the life-cycle reviews. Ultimately, this knowledge will enable the technical team to provide advice to the program/project and ETA as to the suitability of the product for acceptance.";
		}

		requirement def <'SE-31'> SE31 :> SERequirement {
			doc /* The NASA technical team shall participate in product transition as defined in the
			 * NASA SEMP or other equivalent program/project documentation.
			 */
			:>> id = "SE-31";
			:>> section = "4.4.2";
			:>> rationale = "In accordance with the SEMP (or other equivalent program/project documentation), the technical team will participate in the execution of the final aspects of the end product either its transference in whole to the program/ project customer, its operations, and/or the final decommissioning and disposal. These activities may be performed by the same team that was involved in its development or by other technical teams.";
		}

		requirement def <'SE-32'> SE32 :> SERequirement {
			doc /* The technical team shall develop and document plans for life-cycle and technical
			 * reviews for use in the program/project planning process.
			 */
			:>> id = "SE-32";
			:>> section = "5.2.1.1";
			:>> rationale = "Each of the life-cycle reviews, as well as any other technical status reviews, needs to be identified and documented so that all stakeholders will know how the program/ projects progress will be assessed. This will typically be captured within the SEMP, in a separate Review Plan or other equivalent program/project documentation.";
		}

		requirement def <'SE-33'> SE33 :> SERequirement {
			doc /* The technical team shall participate in the life-cycle and technical reviews as
			 * indicated in the governing program/project management NPR.
			 */
			:>> id = "SE-33";
			:>> section = "5.2.1.5";
			:>> rationale = "The technical team will be responsible for generating and presenting many of the technical topics during a life-cycle and technical review.";
		}

		requirement def <'SE-34'> SE34 :> SERequirement {
			doc /* The technical team shall participate in the development of entrance and success
			 * criteria for each of the respective reviews.
			 */
			:>> id = "SE-34";
			:>> section = "5.2.2.1";
			:>> rationale = "The entrance and success criteria in Appendix G are provided as guidelines (not requirements). It is expected that they will be modified as needed by the program/project according to their size, complexity, type of end product being produced, formality, and risk acceptance posture. Specific names of documents may be provided for clarity, non-applicable products eliminated, and new products added as needed for clarity and completeness.";
		}

		requirement def <'SE-35'> SE35 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: MCR: Baselined stakeholder
			 * identification and expectation definitions.
			 */
			:>> id = "SE-35";
			:>> section = "5.2.2.2.a(1)";
			:>> rationale = "For an MCR one of the key products is capturing the stakeholder expectations. These may be identified as needs, goals, and objectives, or other methods for capturing their expectations. These are captured in a document or a database/model. After all comments from the MCR are dispositioned, the set of stakeholder expectations are updated with the approved comments and then baselined.";
		}

		requirement def <'SE-36'> SE36 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: MCR: Baselined concept definition.
			 */
			:>> id = "SE-36";
			:>> section = "5.2.2.2.a(2)";
			:>> rationale = "Presenting one or more feasible ways of accomplishing the stakeholder expectations is a key product of the MCR. These are captured in a document or a database/model. After all comments from the MCR are dispositioned, the concept(s) are updated with the approved comments and then baselined.";
		}

		requirement def <'SE-37'> SE37 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: MCR: Approved Measures of
			 * Effectiveness (MOE) definition.
			 */
			:>> id = "SE-37";
			:>> section = "5.2.2.2.a(3)";
			:>> rationale = "The MOE capture the stakeholders' view of what would be considered the successful achievement of each expectation. These will help in the later identification of requirements, criteria for trade studies and in the success criteria for the validation efforts.";
		}

		requirement def <'SE-38'> SE38 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: SRR: Baselined SEMP (or other
			 * equivalent program/project documentation) for projects, single-project programs, and
			 * one-step AO programs.
			 */
			:>> id = "SE-38";
			:>> section = "5.2.2.2.b(1)";
			:>> rationale = "The SEMP is a key document for the technical effort in a similar manner that the program/project plan captures the programmatic efforts. These are captured in a document or a database/model. For projects, single-project programs, and one-step AO programs after all comments from the SRR are dispositioned, the SEMP (or other equivalent program/project documentation) is updated with the approved comments and then baselined. The SEMP (or other equivalent program/project documentation) is baselined in a later phase for the other types of programs and so will be a 'Not Applicable' in this line for uncoupled, tightly coupled, and loosely coupled programs.";
		}

		requirement def <'SE-39'> SE39 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: SRR: Baselined requirements.
			 */
			:>> id = "SE-39";
			:>> section = "5.2.2.2.b(2)";
			:>> rationale = "The program/project requirements are a key product for the SRR. These are captured in a document or a database/model. After all comments from the SRR are dispositioned, the requirements are updated with the approved comments and then baselined.";
		}

		requirement def <'SE-40'> SE40 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: MDR/SDR: Approved TPM definitions.
			 */
			:>> id = "SE-40";
			:>> section = "5.2.2.2.c(1)";
			:>> rationale = "A key product at the SDR is the set of TPMs that the program/project has identified as the important measures to track for their efforts. These may be associated with the key driving requirements, key performance parameters, leading or lagging indicators, or other measures that are important to periodically measure and track.";
		}

		requirement def <'SE-41'> SE41 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: MDR/SDR: Baselined architecture
			 * definition.
			 */
			:>> id = "SE-41";
			:>> section = "5.2.2.2.c(2)";
			:>> rationale = "One of the key products of an SDR is the proposed architecture that will accomplish the requirements. These are captured in a document or a database/model. After all comments from the SDR are dispositioned, the architecture description is updated with the approved comments and then baselined.";
		}

		requirement def <'SE-42'> SE42 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: MDR/SDR: Baselined allocation of
			 * requirements to next lower level.
			 */
			:>> id = "SE-42";
			:>> section = "5.2.2.2.c(3)";
			:>> rationale = "Now that the overarching architecture has been defined, it is important to show how the requirements are allocated to the architecture elements of the next lower level of the product hierarchy. These are captured in a document or a database/ model. After all comments from the SDR are dispositioned, the allocation is updated with the approved comments and then baselined.";
		}

		requirement def <'SE-43'> SE43 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: MDR/SDR: Initial trend of required
			 * leading indicators.
			 */
			:>> id = "SE-43";
			:>> section = "5.2.2.2.c(4)";
			:>> rationale = "The trend is presented for the leading indicators that have been identified by the Agency as required for each program/project. These will typically be in graphical form but could also be tabular or other form appropriate for the project. At SDR this will be the initial set of trends that have been captured since SRR. Since final hardware has not been produced at this point, the trends will be on the estimated parameters.";
		}

		requirement def <'SE-44'> SE44 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: MDR/SDR: Baselined SEMP (or other
			 * equivalent program/project documentation) for uncoupled, loosely coupled, tightly
			 * coupled, and two-step AO programs.
			 */
			:>> id = "SE-44";
			:>> section = "5.2.2.2.c(5)";
			:>> rationale = "The SEMP is a key document for the technical effort in a similar manner that the program plan captures the programmatic efforts. These are captured in a document or a database/model. For uncoupled, loosely coupled, tightly coupled, and two-step AO programs, after all comments from the MDR/SDR are dispositioned, the SEMP (or other equivalent program/project documentation) is updated with the approved comments and then baselined. The SEMP (or other equivalent program/project documentation) is baselined in an earlier phase for projects and single-project programs and so will be a 'Not Applicable' in this line for those types of programs.";
		}

		requirement def <'SE-45'> SE45 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: PDR: Preliminary design solution
			 * definition.
			 */
			:>> id = "SE-45";
			:>> section = "5.2.2.2.d(1)";
			:>> rationale = "The key product of a PDR is the preliminary design itself. The design is captured in one or more documents, models, databases, drawings, and other means. Comments from the PDR will be captured in the final design for the next review.";
		}

		requirement def <'SE-46'> SE46 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: CDR: Baseline detailed design.
			 */
			:>> id = "SE-46";
			:>> section = "5.2.2.2.e(1)";
			:>> rationale = "The key product of a CDR is the final design. The design is captured in one or more documents, models, databases, drawings, and other means. The final design is updated with approved comments from the review, and the design is updated to represent the design that will be implemented.";
		}

		requirement def <'SE-47'> SE47 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: SIR: Updated integration plan.
			 */
			:>> id = "SE-47";
			:>> section = "5.2.2.2.f(1)";
			:>> rationale = "A key product of an SIR is the updated integration plans. These will describe how the products associated with this review will be integrated.";
		}

		requirement def <'SE-48'> SE48 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: SIR: Initial V&V results.
			 */
			:>> id = "SE-48";
			:>> section = "5.2.2.2.f(2)";
			:>> rationale = "Another key product of an SIR is the initial V&V results from any of the lower level products that are associated with this review. With the recursive nature of the SE engine, products will be integrated and verified/validated from the bottom of the product layer to the top. So, prior to integration into larger assemblies, lower level products will have been through their V&V activities. This ensures that, when they are assembled into the higher product layers, they will work as intended. Programs/projects may decide to perform V&V only at assembly levels as captured in their SEMP (or other equivalent program/project documentation) and so initial V&V results may or may not be available.";
		}

		requirement def <'SE-51'> SE51 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: ORR: Baselined decommissioning
			 * plans.
			 */
			:>> id = "SE-51";
			:>> section = "5.2.2.2.g(2)";
			:>> rationale = "At ORR it is important to describe how the product will ultimately be decommissioned when it has accomplished its mission. This is to ensure that decommissioning will be feasible before the product is put into use. These are captured in a document or a database/model. After all comments from the ORR are dispositioned, the plan is updated with the approved comments and then baselined.";
		}

		requirement def <'SE-52'> SE52 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: ORR: Baselined disposal plans.
			 */
			:>> id = "SE-52";
			:>> section = "5.2.2.2.g(3)";
			:>> rationale = "At ORR it is also important to describe how the product will ultimately be disposed of when it has accomplished its mission. This is to ensure that disposal will be feasible before the product is put into use. These are captured in a document or a database/model. After all comments from the ORR are dispositioned, the plan is updated with the approved comments and then baselined.";
		}

		requirement def <'SE-53'> SE53 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: FRR: Baseline V&V results.
			 */
			:>> id = "SE-53";
			:>> section = "5.2.2.2.h(1)";
			:>> rationale = "At FRR, the baselined V&V results for the product are presented and any remaining open work identified. This is to ensure that the product is ready for flight. Note that for some programs/projects the V&V results may need to be baselined at ORR per Center policies/procedures. Maturing and presenting a product earlier than required in the Agency NPR is at the discretion of the program/project/Center and does not require a waiver.";
		}

		requirement def <'SE-54'> SE54 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: FRR: Final certification for
			 * flight/use.
			 */
			:>> id = "SE-54";
			:>> section = "5.2.2.2.h(2)";
			:>> rationale = "The key product at the FRR is the certification that the product is ready for flight/use. This gains agreement with all key stakeholders that the product is ready to put into the operational phase. Any remaining open items are identified and plans for closure are developed.";
		}

		requirement def <'SE-55'> SE55 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: DR: Updated decommissioning plans.
			 */
			:>> id = "SE-55";
			:>> section = "5.2.2.2.i(1)";
			:>> rationale = "The key product at the DR is the plan on how the product will be removed from service. The approved comments from the DR are used to baseline the plan.";
		}

		requirement def <'SE-56'> SE56 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: DRR: Updated disposal plans.
			 */
			:>> id = "SE-56";
			:>> section = "5.2.2.2.j(1)";
			:>> rationale = "The key product of the DRR is the plan on how the product will be disposed of after it has been decommissioned. The approved comments from the DRR are used to update the plan.";
		}

		requirement def <'SE-57'> SE57 :> SERequirement {
			doc /* Technical teams shall monitor technical effort through periodic technical reviews.
			 */
			:>> id = "SE-57";
			:>> section = "5.2.2.7";
			:>> rationale = "In addition to the life-cycle reviews, the technical teams need to periodically monitor the technical progress of their program/project. These may be held formally or informally with relevant personnel.";
		}

		requirement def <'SE-58'> SE58 :> SERequirement {
			doc /* The technical teams shall define in the program/project SEMP how the required 17
			 * common technical processes, as tailored, will be recursively applied to the various
			 * levels of program/project product layer system structure during each applicable life-
			 * cycle phase.
			 */
			:>> id = "SE-58";
			:>> section = "6.2.3";
			:>> rationale = "The SEMP is the key document that lays out the work that the technical team needs to perform and the manner in which they will perform it. This requirement ensures that each of the 17 common technical processes is addressed and how it will be applied to the various levels in the end-item product hierarchy and their associated enabling products.";
		}

		requirement def <'SE-59'> SE59 :> SERequirement {
			doc /* The technical team shall ensure that any technical plans and discipline plans are
			 * consistent with the SEMP (or equivalent program/project documentation) and are
			 * accomplished as fully integrated parts of the technical effort.
			 */
			:>> id = "SE-59";
			:>> section = "6.2.5";
			:>> rationale = "Since the SEMP is the primary planning document for the SE effort, all subsequent planning documents are in alignment and consistent with the SEMP.";
		}

		requirement def <'SE-60'> SE60 :> SERequirement {
			doc /* The technical team shall establish TPMs for the program/project that track/describe
			 * the current state versus plan.
			 */
			:>> id = "SE-60";
			:>> section = "6.2.6";
			:>> rationale = "The measures that the program/project will use to track the progress of key aspects of the technical effort are identified and documented. These TPMs will include the required leading indicators described in other requirements of this NPR and also any project-unique measures deemed necessary to track the key performance parameters. The selected TPMs need to be measured periodically and their trends reported to the program/project manager at the agreed-to interval as documented in the SEMP (or other equivalent program/project documentation). This ensures the PM and ETA are kept up to date on these key parameters so that decisions can be made in a timely manner.";
		}

		requirement def <'SE-61'> SE61 :> SERequirement {
			doc /* The technical team shall report the TPMs to the Program/Project Manager on an agreed-
			 * to reporting interval.
			 */
			:>> id = "SE-61";
			:>> section = "6.2.7";
			:>> rationale = "The measures that the program/project will use to track the progress of key aspects of the technical effort are identified and documented. These TPMs will include the required leading indicators described in other requirements of this NPR and also any project-unique measures deemed necessary to track the key performance parameters. The selected TPMs need to be measured periodically and their trends reported to the program/project manager at the agreed-to interval as documented in the SEMP (or other equivalent program/project documentation). This ensures the PM and ETA are kept up to date on these key parameters so that decisions can be made in a timely manner.";
		}

		requirement def <'SE-62'> SE62 :> SERequirement {
			doc /* The technical team shall ensure that the set of TPMs include the following leading
			 * indicators: a. Mass margins for projects involving hardware.
			 */
			:>> id = "SE-62";
			:>> section = "6.2.8.a";
			:>> rationale = "If the program/project has hardware elements, tracking of the remaining margins associated with their mass is a required leading indicator measure by the Agency. This is especially important for flight projects. For ground or other projects in which mass is not relevant, a waiver to this requirement can be documented in the SEMP or other equivalent program/project documentation.";
		}

		requirement def <'SE-63'> SE63 :> SERequirement {
			doc /* The technical team shall ensure that the set of TPMs include the following leading
			 * indicators: b. Power margins for projects that are powered.
			 */
			:>> id = "SE-63";
			:>> section = "6.2.8.b";
			:>> rationale = "If the program/project has elements that require power, tracking of the remaining margins associated with their power consumption is a required leading indicator measure by the Agency. This is especially important for flight projects. For ground or other projects in which power consumption is not relevant, a waiver to this requirement can be documented in the SEMP or other equivalent program/project documentation.";
		}

		requirement def <'SE-64'> SE64 :> SERequirement {
			doc /* The technical team shall ensure that a set of review trends is created and maintained
			 * that includes closure of review action documentation (RIDs, RFAs, and/or Action Items
			 * as established by the project).
			 */
			:>> id = "SE-64";
			:>> section = "6.2.9";
			:>> rationale = "During life-cycle reviews, comments from the reviewers are captured on forms, databases, spreadsheets, or other manner. Depending on the program/project, these may be called RFAs, RIDs, Action Items, or other terminology. Whatever they are called, the disposition and closure of these comments typically called their burndown are required indicator trends by the Agency. This ensures that the approved comments are incorporated into the designs and plans in a timely manner.";
		}

		requirement def <'SE-65'> SE65 :> SERequirement {
			doc /* The technical team shall develop and document an approach to Human Systems
			 * Integration (HSI).
			 */
			:>> id = "SE-65";
			:>> section = "5.2.1.3";
			:>> rationale = "The HSI approach ensures that system aspects that interface with humans (e.g., crew, operators, users, maintainers, assemblers, and ground support personnel) have been considered and that HSI requirements will be met.";
		}

		requirement def <'SE-66'> SE66 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: SRR: Baselined Human Systems
			 * Integration approach
			 */
			:>> id = "SE-66";
			:>> section = "5.2.2.2.b(3)";
			:>> rationale = "An important product of the SRR is a baselined approach to HSI. This ensures that human interfaces with the products of the project have been considered and that the HSI requirements from this review will be addressed.";
		}

		requirement def <'SE-67'> SE67 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: PDR: Baselined integration plans
			 */
			:>> id = "SE-67";
			:>> section = "5.2.2.2.d(2)";
			:>> rationale = "A key product of an SIR are the integration plans. These will describe how the products of this project will be integrated. Depending on the complexity of the project this may be a stand-alone plan or incorporated into another document like the SEMP or the V&V Plan.";
		}

		requirement def <'SE-68'> SE68 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: PDR: Baselined Verification and
			 * Validation Plan
			 */
			:>> id = "SE-68";
			:>> section = "5.2.2.2.d(3)";
			:>> rationale = "At PDR the plan of how the requirements of the project products will be verified and how the stakeholder expectations of those products will be validated is baselined. This ensures that the project and its stakeholders are in agreement of how the V&V will proceed and that the results will provide adequate information to decide upon the acceptability of the design.";
		}

		requirement def <'SE-69'> SE69 :> SERequirement {
			doc /* The technical team shall provide the following minimum products at the associated
			 * life-cycle review at the indicated maturity level: ORR: Preliminary V&V results
			 */
			:>> id = "SE-69";
			:>> section = "5.2.2.2.g(1)";
			:>> rationale = "At ORR the project should have at least preliminary results from the V&V activities. Any remaining requirements or expectations that have not been verified or validated should be discussed at the ORR.";
		}
	}

	attribute def ComplyCode {
		doc /* The code a ComplyKind value writes in the 'Comply?' column of Table H-1. */
		attribute code : String;
	}

	enum def ComplyKind :> ComplyCode {
		doc /* The 'Comply?' column of Table H-1: 'FC' fully compliant, 'T' tailored, 'NA' not
		 * applicable.
		 */
		fullyCompliant {
			doc /* FC: the program/project complies with the requirement as written. */
			:>> code = "FC";
		}
		tailored {
			doc /* T: the program/project tailored the requirement, by waiver or deviation (section
			 * 2.2.1); the justification column records why.
			 */
			:>> code = "T";
		}
		notApplicable {
			doc /* NA: the requirement does not apply to the program/project; the justification column
			 * records why.
			 */
			:>> code = "NA";
		}
	}

	metadata def <complies> Compliance {
		doc /* One entry of the compliance matrix: how the program/project complies with the annotated
		 * SE requirement usage and, when tailored or not applicable, the justification Table H-1
		 * asks for. Section 2.1 makes the program/project and the ETA responsible 'for completing
		 * the compliance matrix in Appendix H, capturing any tailoring, and including it in the
		 * SEMP'. `requirement se62 : SERequirements::SE62 { @Compliance { comply =
		 * ComplyKind::tailored; justification = "..."; } }`
		 */
		:> annotatedElement : SysML::RequirementUsage;
		:> annotatedElement : SysML::RequirementDefinition;
		attribute comply : ComplyKind;
		attribute justification : String[0..1];
	}

	calc def ComplianceEntries :> Query {
		doc /* The SE requirement usages below root that carry a Compliance annotation. */
		in root : Element;
		in depth : Integer default 16;
		WhereMetadata(
			source = Descendants(source = root, maxDepth = depth),
			'metadata' = "NPR7123::Compliance"
		)
	}

	calc def ComplianceRegister :> Query {
		doc /* Table H-1 generated from the project model: one row per compliance entry below root with
		 * the Req ID, NPR Section, Requirement Statement (the documentation of the usage's
		 * definition), Rationale, Comply? value and Justification, in the columns of Table H-1.
		 */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = ComplianceEntries(root = root, depth = depth),
			columns = (
				Column(name = "Req ID", path = "id"),
				Column(name = "NPR Section", path = "section"),
				Column(name = "Requirement Statement", path = "type.documentation"),
				Column(name = "Rationale", path = "rationale"),
				Column(name = "Comply?", path = "NPR7123::Compliance::comply"),
				Column(name = "Justification", path = "NPR7123::Compliance::justification")
			)
		)
	}

	calc def LifeCycleReviewRegister :> Query {
		doc /* One row per life-cycle review below root: name, kind, phase, status and documentation.
		 */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = depth), type = "NPR7123::LifeCycleReview"),
			properties = ("name", "kind", "phase", "status", "documentation"),
			tolerate = true
		)
	}

	abstract part def ComplianceMatrix :> Document {
		doc /* The compliance matrix of Appendix H for a program/project. Specialize it and add a table
		 * whose rows run ComplianceRegister over the project's root: `part def
		 * ProjectComplianceMatrix :> ComplianceMatrix { part entries : Section { part matrix :
		 * Table { calc rows : ComplianceRegister { in :>> root = Project; } } } }`. A complete
		 * matrix has one compliance entry per active SE requirement; the project annotates a usage
		 * of each with `@Compliance`.
		 */
		:>> title default = "NPR 7123.1 Compliance Matrix";
	}

	view def ComplianceMatrixView :> GridView {
		doc /* Table of the compliance entries, one row per SE requirement usage that carries a
		 * Compliance annotation.
		 */
		filter @Compliance;
		render asElementTable;
	}

	view def LifeCycleReviewView :> GridView {
		doc /* Table of the life-cycle reviews. */
		filter @LifeCycleReviewMetadata;
		render asElementTable;
	}

	/* Appendix G: entrance and success criteria, one requirement definition per criterion */

	package ReviewCriteria {
		doc /* The entrance and success criteria of NPR 7123.1D Appendix G, Tables G-1 to G-21,
		 * transcribed one requirement definition per criterion and numbered as in the tables.
		 * Section G.1.2 and section 5.1.3 expect tailoring: 'Programs/projects are expected to
		 * tailor the reviews and customize the entrance/success criteria as appropriate to the
		 * size/complexity and unique needs of their activities. Approved tailoring is captured in
		 * the SEMP or other equivalent program/project documents.' A project subsets the
		 * definitions it adopts into its review's `entranceCriteria` and `successCriteria`,
		 * redefines the documentation of the ones it customizes, and adds its own. A leading '*' in
		 * a criterion marks a product 'required for programs/projects covered by NPR 7120.5. If
		 * there is disagreement between this table and NPR 7120.5, NPR 7120.5 takes precedence.'
		 */

		package ProgramSRR {
			doc /* Table G-1: System Requirements Review for Programs entrance and success criteria.
			 */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The Program has successfully completed the MCR life-cycle review (if applicable)
				 * and all RFAs and RIDs have been addressed and resolved, or a timely closure plan
				 * exists for those remaining open.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* A preliminary Program SRR agenda, success criteria, and instructions to the
				 * review board have been agreed to by the technical team, the program manager, and
				 * the review chair prior to the Program SRR.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* All planned higher level SRRs and peer reviews have been successfully conducted
				 * and RID/RFA/Action Items have been addressed with the originator or designated
				 * TA.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Top program risks with significant technical, health and medical, system security
				 * (including cybersecurity), safety, cost, and schedule impacts have been
				 * identified along with corresponding mitigation strategies.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* An approach for verifying compliance with program requirements has been defined.
				 */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* Procedures for controlling changes to program requirements have been defined and
				 * approved.
				 */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* The following primary products are ready for review: a. Program requirements
				 * (including performance, health and medical, safety, human system integration, and
				 * defined external system interfaces to other programs) are ready to be baselined
				 * after review comments are incorporated. b. Baselined approach for human systems
				 * integration.
				 */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* Other program SRR technical products have been made available to the cognizant
				 * participants prior to the review: a. *Preliminary traceability of program-level
				 * requirements on projects to the Agency strategic goals and Mission Directorate
				 * requirements and constraints. b. *Initial risk mitigation plans and resources for
				 * significant technical risks. c. *Preliminary cost and schedule for uncoupled,
				 * loosely coupled, and tightly coupled programs. d. *Preliminary documentation of
				 * Basis of Estimate (cost and schedule) for uncoupled, loosely coupled, and tightly
				 * coupled programs. e. *Review Plan ready to be baselined after review comments are
				 * incorporated. f. *Preliminary Configuration Management Plan. g. *Preliminary SEMP
				 * (or equivalent program documentation) for uncoupled, loosely coupled, tightly
				 * coupled, and two-step AO programs. h. RF (radio frequency) spectrum requirements
				 * have been identified. i. Preliminary System Security Plan.
				 */
				:>> number = 9;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Program requirements have been defined and support Mission Directorate strategic
				 * objectives.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The program requirements are adequately levied on projects of the program.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* Traceability of program requirements to individual projects is documented in
				 * accordance with Agency needs, goals, and objectives, as described in the NASA
				 * Strategic Plan.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Definition of external system interfaces with other programs is adequately mature
				 * and approved.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* The program cost and schedule estimates are credible to meet program
				 * requirements.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Top risk identification is complete and mitigation strategies appear reasonable.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Evidence is provided that the program is compliant with NASA and implementing
				 * Center requirements, standards, processes, and procedures.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* To-be-determined (TBD) and to-be-resolved (TBR) items are clearly identified with
				 * acceptable plans and schedules for their disposition.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* The responsible Center spectrum manager at the responsible Center was notified of
				 * preliminary requirements.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* Proposed tailoring is appropriate and consistent with applicable Agency and
				 * Center guidance.
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Lessons Learned from other projects and programs have been identified and
				 * addressed.
				 */
				:>> number = 11;
			}
		}

		package ProgramSDR {
			doc /* Table G-2: System Definition Review for Programs entrance and success criteria.
			 */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The program has successfully completed the previous planned life-cycle reviews
				 * and all RFAs and RIDs have been addressed and resolved, or a timely closure plan
				 * exists for those remaining open.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* An agenda for the program SDR, success criteria, and instructions to the review
				 * board have been agreed to by the technical team, the project manager, and the
				 * review chair prior to the review.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* All planned higher level SDRs and peer reviews have been successfully conducted
				 * and RID/RFA/Action Items have been addressed with the originator or designated
				 * TA.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* The following primary products are ready for review: a. Approved definition of
				 * program TPMs. b. Program architecture definition and a list of specific
				 * supporting projects that are ready to be baselined after review comments are
				 * incorporated. c. Allocation of program requirements to the supporting projects
				 * that is ready to be baselined after review comments are incorporated. d. Approval
				 * and status of technical performance related to leading indicators, margins, TPMs,
				 * and resolution of the previous review discrepancies addressing effectiveness of
				 * technical achievement and communicating the overall risk to the project. e. SEMP
				 * (or equivalent program documentation) ready to be baselined for uncoupled,
				 * tightly coupled, and loosely coupled programs and for two-step AO programs.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Other SDR technical products (as applicable) for hardware, software, and human
				 * system elements have been made available to the cognizant participants prior to
				 * the review: a. *Updated program requirements and constraints. b. *Traceability of
				 * program-level requirements on projects to the Agency strategic goals and Mission
				 * Directorate requirements and constraints that is ready to be baselined after
				 * review comments are incorporated. c. Preliminary system interface definitions. d.
				 * Preliminary implementation plans. e. Preliminary integration plans. f.
				 * *Preliminary verification and validation plans. g. *Updated cost and schedule. h.
				 * *Updated SEMP (or equivalent program documentation) for one-step AO programs and
				 * single-project programs. i. *Updated risk mitigation plans and resources for
				 * significant technical risks. j. *Updated cost and schedule. k. *Updated
				 * Documentation of Basis of Estimate (cost and schedule). l. *Preliminary plans for
				 * technical work to be accomplished during Implementation. m. *Updated Review Plan.
				 * n. *Configuration Management Plan that is ready to be baselined after review
				 * comments are incorporated. o. Preliminary assessment of RF spectrum requirements.
				 * p. *Baseline System Security Plan for uncoupled and loosely coupled programs.
				 */
				:>> number = 6;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Evidence is provided that the program formulation activities are complete and
				 * implementation plans are credible to meet mission success.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The program requirements address critical NASA needs as identified in the Mission
				 * Directorate strategic objectives.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The program cost and schedule estimates are credible to meet program requirements
				 * within available resources.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Program implementation plans are credible to achieve mission success. */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* The program risks have been identified and mitigation strategies appear
				 * reasonable.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Allocation of program requirements to projects has been completed and proposed
				 * projects are feasible within available resources.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* The maturity of the programâ??s definition and associated plans is sufficient to
				 * begin preliminary design.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* The program has demonstrated compliance with applicable NASA and implementing
				 * Center requirements, standards, processes, and procedures.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* TBD and TBR items are clearly identified with acceptable plans and schedules for
				 * their disposition.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* Program has clearly identified plans and schedules for applicable RF system
				 * certification data package submissions (experimental, developmental, or
				 * operational).
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Center spectrum manager at responsible Center was notified of preliminary
				 * requirement assessment.
				 */
				:>> number = 11;
			}
		}

		package MCR {
			doc /* Table G-3: Mission Concept Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* An agenda for the MCR, success criteria, and instructions to the review board
				 * have been agreed to by the technical team, the project manager, and the review
				 * chair prior to the review.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* All planned higher level MCRs and peer reviews have been successfully conducted
				 * and RID/RFA/Action Items have been addressed and resolved with the originator or
				 * designated TA, or a timely closure plan exists for those remaining open.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* The following primary products are ready for review: a. Stakeholders have been
				 * identified and stakeholder expectations have been defined and are ready to be
				 * baselined after review comments are incorporated. b. The concept has been
				 * developed to a sufficient level of detail to demonstrate a technically feasible
				 * solution to the mission/project needs and is ready to be baselined after review
				 * comments are incorporated. c. MOEs and any other mission success criteria have
				 * been defined and are ready to be approved.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Other technical products (as applicable) for hardware, software, and human system
				 * elements have been made available to the cognizant participants prior to the
				 * review: a. *Mission/project goals and objectives that are ready to be baselined
				 * after review comments are incorporated. b. Alternative concepts that have been
				 * analyzed and are ready to be reviewed. c. *Initial risk-informed cost and
				 * schedule estimates for implementation. d. *Preliminary mission descope options.
				 * e. *A preliminary assessment performed by the team of top technical, cost,
				 * schedule, system security (including cybersecurity), and safety risks with
				 * developed associated risk management and mitigation strategies and options. f.
				 * *Preliminary approach to verification and validation for the selected concept(s).
				 * g. *A preliminary SEMP (or equivalent project documentation), including technical
				 * plans. h. *Technology Development Plan that is ready to be baselined after review
				 * comments are incorporated. i. *Initial technology readiness that has been
				 * assessed and documented with technology assets, heritage products, and gaps
				 * identified. j. Single Point Failure/Fault Tolerance philosophy. k. Preliminary
				 * engineering development assessment and technical plans to achieve what needs to
				 * be accomplished in the next phase. l. Conceptual life-cycle support strategies
				 * (logistics, supply chain management, manufacturing, and operation). m. Software
				 * criteria and products, per NASA-HDBK-2203. n. Preliminary assessment of RF
				 * spectrum needs. o. Preliminary Human Systems Integration approach. (For
				 * additional guidance on HSI entrance criteria refer to NASA/SP-20210010952, NASA
				 * Human Systems Integration (HSI) Handbook).
				 */
				:>> number = 5;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Mission objectives are clearly defined and stated and are unambiguous and
				 * internally consistent.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The selected concept(s) satisfactorily meets the stakeholder expectations.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The mission is feasible. A concept has been identified that is technically and
				 * logistically feasible. A rough cost estimate is within an acceptable cost range.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* The concept evaluation criteria to be used in candidate systems evaluation have
				 * been identified and prioritized.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* The need for the mission has been clearly identified. */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* The cost and schedule estimates are credible and sufficient resources are
				 * available for project formulation.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* The program/project has demonstrated compliance with applicable NASA and
				 * implementing Center requirements, standards, processes, and procedures.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* TBD and TBR items are clearly identified with acceptable plans and schedule for
				 * their disposition.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* Alternative concepts have adequately considered the use of existing assets or
				 * products that could satisfy the mission or parts of the mission.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* Technical planning is sufficient to proceed to the next phase and includes
				 * planning for hardware, software, human systems, and data deliverables.
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Risk and mitigation strategies have been identified and are acceptable based on
				 * technical risk assessments.
				 */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in the NASA-HDBK-2203.
				 */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* Human Systems Integration aspects are included in the management and technical
				 * planning following guidelines within NASA/SP-20210010952 and are sufficient to
				 * proceed to the next phase.
				 */
				:>> number = 13;
			}

			requirement def Success14 :> SuccessCriterion {
				doc /* Concurrence by the responsible Center spectrum manager that RF needs have been
				 * properly identified and addressed.
				 */
				:>> number = 14;
			}
		}

		package SRR {
			doc /* Table G-4: System Requirements Review for Projects and Single-Project Programs
			 * entrance and success criteria.
			 */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The project has successfully completed the previously planned life-cycle reviews
				 * and responses have been made to all RFAs and RIDs, or a timely closure plan
				 * exists for those items remaining open.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* A preliminary SRR agenda, success criteria, and instructions to the review board
				 * have been agreed to by the technical team, project manager, and review chair
				 * prior to the SRR.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* All planned higher level SRR and peer reviews have been successfully conducted
				 * and RID/RFA/Action Items have been addressed and resolved with the originator or
				 * designated TA, or a timely closure plan exists for those remaining open.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* The following primary technical products for hardware, software and human system
				 * elements are available to the cognizant participants prior to the review: a.
				 * Requirements for system being reviewed are ready to be baselined after the review
				 * and preliminary allocation to the next lower level system has been performed. b.
				 * For projects, one-step AO programs and single-project programs, the SEMP (or
				 * equivalent program/project documentation) is ready to be baselined after review
				 * comments are incorporated. c. Human Systems Integration approach is ready to be
				 * baselined after review comments are incorporated. (For additional guidance on HSI
				 * entrance criteria refer to NASA/SP-20210010952, NASA Human Systems Integration
				 * (HSI) Handbook)
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Other SRR work products (as applicable) for hardware, software, and human system
				 * elements have been made available to the cognizant participants. a. *Updated
				 * concept definition. b. *Updated concept of operations. c. Updated parent
				 * requirements. d. *Risk management plan ready to be baselined after review
				 * comments are incorporated. e. *Updated risk assessment and mitigations. f.
				 * *Configuration management plan ready to be baselined after review comments are
				 * incorporated. g. Initial document tree or model structure. h. Preliminary
				 * verification and validation method identified for each requirement. i.
				 * Preliminary system safety analysis. j. Product certification or product
				 * acceptance data requirements. k. Interfaces with external systems are identified
				 * and preliminary definitions are ready to be baselined (e.g., Interface Control
				 * Documents). l. Preliminary MOPS and TPM and other key driving requirements. m.
				 * Other specialty discipline analyses, as required. n. *Updated cost and schedule
				 * estimates for the project implementation. o. *Updated documentation of Basis of
				 * Estimate (cost and schedule). p. *Updated Technology Development Plan. q.
				 * *Updated technology readiness assessment that has been reviewed and documented
				 * that includes technology assets, heritage products, and capability gaps
				 * identified. r. Logistics documentation (e.g., preliminary maintenance plan). s.
				 * *Initial Human Rating Certification Package. t. *System safety and mission
				 * assurance plan ready to be baselined after review comments are incorporated. u.
				 * *Preliminary operations concept. v. Preliminary engineering development
				 * assessment and technical plans to achieve what needs to be accomplished in the
				 * next phase. w. Software criteria and products, per the NASA-HDBK-2203. x. RF
				 * spectrum requirements have been addressed including preparing requisite data for
				 * the responsible Center Spectrum Manager for possible Stage 1 Certification. y.
				 * *Preliminary System Security Plan.
				 */
				:>> number = 6;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The functional and performance requirements defined for the system are responsive
				 * to the stakeholder needs and parent requirements, reflect the system's intended
				 * operational use, and represent capabilities likely to be achieved within the
				 * scope of the project.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The maturity of the requirements definition and associated plans is sufficient to
				 * begin Phase B.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The project utilizes a sound process for the allocation and control of
				 * requirements throughout all levels, and a plan has been defined to complete the
				 * requirements definition at lower levels within schedule constraints.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* System Interfaces with external entities and between major internal elements have
				 * been identified, including system security expectations.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* Preliminary approaches have been determined for how requirements will be verified
				 * and validated.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Major risks have been identified and technically assessed, and viable mitigation
				 * strategies have been defined.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* The program/project has demonstrated compliance with applicable NASA and
				 * implementing Center requirements, standards, processes, and procedures.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* TBD and TBR items are clearly identified with acceptable plans and schedule for
				 * their disposition.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in NASA-HDBK-2203. */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* Human Systems Integration aspects are included in the management and technical
				 * planning following guidelines within NASA/SP-20210010952 and are sufficient to
				 * proceed to the next phase
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Concurrence by the responsible Center spectrum manager that the program/project
				 * has provided requisite RF system data.
				 */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* Proposed tailoring is appropriate and consistent with applicable Agency and
				 * Center guidance.
				 */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* Lessons Learned from other projects and programs have been identified and
				 * addressed.
				 */
				:>> number = 13;
			}

			requirement def Success14 :> SuccessCriterion {
				doc /* Single Point Failure/Fault Tolerance philosophy is reflected in requirements.
				 */
				:>> number = 14;
			}
		}

		package MDR_SDR {
			doc /* Table G-5: Mission Definition Review/System Definition Review for Projects and
			 * Single-Project Programs entrance and success criteria.
			 */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The project has successfully completed the previously planned life-cycle reviews
				 * and all RFAs and RIDs have been addressed and resolved, or a timely closure plan
				 * exists for those items remaining open.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* A preliminary MDR/SDR agenda, success criteria, and instructions to the review
				 * board have been agreed to by the technical team, project manager, and review
				 * chair prior to the MDR/SDR.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* All planned higher level MDR/SDR and peer reviews have been successfully
				 * conducted and RID/RFA/Action Items have been addressed with the originator or
				 * designated TA.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* The following primary technical products for hardware, software, and human system
				 * elements are available to the cognizant participants prior to the review: a.
				 * Defined architecture, including major tradeoffs and options ready to be baselined
				 * after review comments are incorporated. b. Allocation of requirements to next
				 * lower level is ready to be baselined after review comments are incorporated. c.
				 * MOPs, TPMs, and other key driving requirements ready to be approved. d. Approval
				 * and status of technical performance related to leading indicators, margins, TPMs,
				 * and resolution of the previous review discrepancies addressing effectiveness of
				 * technical achievement and communicating the overall risk to the project.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Other MDR/SDR technical products listed below for both hardware and software
				 * system elements have been made available to the cognizant participants prior to
				 * the review: a. Supporting analyses, functional/timing descriptions, and
				 * allocations of functions to architecture elements. b. *Updated SEMP (or
				 * equivalent program/project documentation). c. *Updated risk management plan. d.
				 * *Updated risk assessment and mitigations (if required by the governing PM NPR,
				 * including PRA). e. *Updated Technology Development Plan. f. *Updated technology
				 * readiness that has been assessed and documented with technology assets, heritage
				 * products, and gaps identified. g. *Updated cost and schedule data with ranges and
				 * a basis of the estimates. h. *Preliminary Integrated Logistics Support Plan
				 * (ILSP). i. *Updated Human Systems Integration approach . (For additional guidance
				 * on HSI entrance criteria refer to NASA/SP-20210010952, NASA Human Systems
				 * Integration (HSI) Handbook). j. *Updated Human Rating Certification Package. k.
				 * Preliminary system interface definitions. l. Initial technical resource
				 * utilization estimates and margins. m. *Updated safety and mission assurance (SMA)
				 * plan. n. *Preliminary operations concept. o. Preliminary system safety analysis.
				 * p. Software criteria and products, per NASA-HDBK-2203. q. RF spectrum
				 * considerations assessment. r. *Preliminary System Security Plan. s. Preliminary
				 * integration plans t. Preliminary Verification and Validation Plans
				 */
				:>> number = 6;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The proposed mission/system architecture is credible and responsive to program
				 * requirements and constraints, including resources.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The program/project cost and schedule estimates are credible to meet
				 * program/project requirements within available resources with acceptable risk.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The projectâ??s mission/system definition and associated plans are sufficiently
				 * mature to begin Phase B.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* All technical requirements are allocated to the architectural elements. */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* The architecture tradeoffs are completed, and those planned for Phase B
				 * adequately address the option space.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Significant development, mission, and health and medical safety risks are
				 * identified and technically assessed, and a process and resources exist to manage
				 * the risks.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Adequate planning exists for the development, insertion, or deployment of any
				 * enabling new technology.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* The operations concept is consistent with proposed design concept(s) and is in
				 * alignment with the mission requirements.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* The program/project has demonstrated compliance with applicable NASA and
				 * implementing Center requirements, standards, processes, and procedures.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* TBD and TBR items are clearly identified with acceptable plans and schedule for
				 * their disposition.
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in NASA-HDBK-2203. */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* Human Systems Integration aspects are included in the management and technical
				 * planning following guidelines within NASA/SP-20210010952 and are sufficient to
				 * proceed to the next phase
				 */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* Concurrence by the responsible Center spectrum manager that RF spectrum
				 * considerations have been addressed.
				 */
				:>> number = 13;
			}

			requirement def Success14 :> SuccessCriterion {
				doc /* Procurement and supply chain risk management execution is complementary with the
				 * technical development schedule.
				 */
				:>> number = 14;
			}

			requirement def Success15 :> SuccessCriterion {
				doc /* Architecture supports the Single Point Failure/Fault Tolerance requirements.
				 */
				:>> number = 15;
			}
		}

		package PDR {
			doc /* Table G-6: Preliminary Design Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The Project has successfully completed the previous planned life-cycle reviews,
				 * and all RFAs and RIDs have been addressed and resolved, or a timely closure plan
				 * exists for those remaining open.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* A preliminary PDR agenda, success criteria, and instructions to the review board
				 * have been agreed to by the technical team, project manager, and review chair
				 * prior to the PDR.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* All planned lower level PDRs and peer reviews have been successfully conducted,
				 * and RID/RFA/Action Items have been addressed with the originator or designated
				 * TA.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* The following primary products are ready for review: a. A preliminary design that
				 * can be shown to meet all technical requirements and performance measures or has
				 * waivers. b. Baselined integration plans c. Baselined Verification and Validation
				 * Plan
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Other PDR technical work products (as applicable) for hardware, software, and
				 * human system elements have been made available to the cognizant participants
				 * prior to the review: a. Subsystem design specifications (hardware and software),
				 * with supporting trade-off analyses and data, as required, that are ready to be
				 * baselined after review comments are incorporated. b. Status of technical
				 * performance related to margins, TPMs, and resolution of the previous review
				 * discrepancies addressing effectiveness of technical achievement and communicating
				 * the overall risk to the project. c. *Updated technology readiness assessment. d.
				 * *Updated Technology Development Plan. e. *Updated risk assessment and mitigation.
				 * f. *Life-Cycle Cost and Integrated Master Schedule (IMS) that are ready to be
				 * baselined after review comments are incorporated. When required, the Joint
				 * Confidence Level (JCL) analysis. g. *Baselined Integrated Logistics Support Plan
				 * (ILSP). h. *Baselined Project Protection Plan. i. Applicable technical plans that
				 * are ready to be baselined after review comments are incorporated (e.g., technical
				 * performance measurement plan, contamination control plan, parts management plan,
				 * environments control plan, Electromagnetic Interference/ Electromagnetic
				 * Compatibility (EMI/EMC) control plan, payload-to-carrier integration plan,
				 * producibility/manufacturability program plan, reliability program plan, quality
				 * assurance plan). j. Applicable design standards that have been identified and
				 * incorporated. k. *Updated safety analyses and plans. l. Preliminary engineering
				 * drawing tree. m. Interface control documents that are ready to be baselined after
				 * review comments are incorporated. n. *Verification/validation plan that is ready
				 * to be baselined after review comments are incorporated. o. Plans to respond to
				 * regulatory requirements (e.g., Environmental Impact Statement), as required, that
				 * are ready to be baselined after review comments are incorporated. p. Preliminary
				 * Disposal Plan. q. Updated technical resource utilization estimates and margins.
				 * r. *Baseline operations concept. s. Updated SEMP (or equivalent program/project
				 * documentation). t. Updated Human Systems Integration approach. (For additional
				 * guidance on HSI entrance criteria refer to NASA/SP-20210010952, NASA Human
				 * Systems Integration (HSI) Handbook). u. *Updated Human Rating Certification
				 * Package. v. Software criteria and products, per NASA-HDBK-2203. w. Design and
				 * requisite data submitted to Center/facility spectrum manager for preparation of
				 * request for certification of Stage 2 spectrum support by at least 60 days prior
				 * to PDR. x. *Updated Preliminary System Security Plan. y. Procurement status
				 * including Supply Chain Risk Management (SCRM) activities (e.g., audits and
				 * assessments, GIDEP, counterfeit avoidance). a`. List of potential single point
				 * failures.
				 */
				:>> number = 6;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The top-level requirements, including mission success criteria, TPMs, and any
				 * sponsor-imposed constraints, are agreed upon, finalized, stated clearly, and
				 * consistent with the preliminary design.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The flow down of verifiable requirements is complete and proper, or, if not, an
				 * adequate plan exists for timely resolution of open items. Requirements are
				 * traceable to parent technical requirements and to mission goals and objectives.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The program/project cost, schedule, and JCL analysis (when required) are credible
				 * and within program/project constraints; are ready for NASA commitment; and are
				 * ready for the Management Agreement (for projects governed by NPR 7120.5).
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* The preliminary design is expected to meet the requirements at an acceptable
				 * level of risk.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* Definition of the system interfaces (both external entities and between internal
				 * elements) is consistent with the overall technical maturity. Associated risks,
				 * including system security, have been identified and represent an acceptable level
				 * of risk.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Any required new technology has been developed to an adequate state of readiness,
				 * or backup options exist and are supported to make them viable alternatives.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* The project risks are understood and have been credibly assessed, and plans, a
				 * process, and resources exist to effectively manage them.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* Safety and mission assurance (e.g., safety, reliability, maintainability, quality
				 * controls, quality verifications, supplier risk management, and Electrical,
				 * Electronic, and Electromechanical (EEE) parts) have been adequately addressed in
				 * preliminary designs and any applicable SMA products (e.g., PRA, system safety
				 * analysis, and failure modes and effects analysis) meet requirements, are at the
				 * appropriate maturity level for this phase of the program/project life cycle, and
				 * indicate that the program/project safety/reliability residual risks will be at an
				 * acceptable level.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* Adequate technical and programmatic margins (e.g., mass, power, memory) and
				 * resources exist to complete the development within budget, schedule, and known
				 * risks.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* The operational concept is technically sound, includes (where appropriate) human
				 * systems, and includes the flow down of requirements for its execution.
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Technical trade studies are mostly complete to sufficient detail and remaining
				 * trade studies are identified, plans exist for their closure, and potential
				 * impacts are understood.
				 */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* The program/project has demonstrated compliance with applicable NASA and
				 * implementing Center requirements, standards, processes, and procedures.
				 */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* TBD and TBR items are clearly identified with acceptable plans and schedule for
				 * their disposition.
				 */
				:>> number = 13;
			}

			requirement def Success14 :> SuccessCriterion {
				doc /* Preliminary analysis of the primary subsystems has been completed and summarized,
				 * highlighting performance and design margin challenges.
				 */
				:>> number = 14;
			}

			requirement def Success15 :> SuccessCriterion {
				doc /* Appropriate modeling and analytical results are available and have been
				 * considered in the design.
				 */
				:>> number = 15;
			}

			requirement def Success16 :> SuccessCriterion {
				doc /* Heritage designs have been suitably assessed for applicability and
				 * appropriateness.
				 */
				:>> number = 16;
			}

			requirement def Success17 :> SuccessCriterion {
				doc /* Manufacturability has been adequately included in design. */
				:>> number = 17;
			}

			requirement def Success18 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in NASA-HDBK-2203. */
				:>> number = 18;
			}

			requirement def Success19 :> SuccessCriterion {
				doc /* Human Systems Integration aspects are included in the management and technical
				 * planning following guidelines within NASA/SP-20210010952 and are sufficient to
				 * proceed to the next phase
				 */
				:>> number = 19;
			}

			requirement def Success20 :> SuccessCriterion {
				doc /* Concurrence by the responsible Center spectrum manager that the program/project
				 * has provided requisite RF system data.
				 */
				:>> number = 20;
			}

			requirement def Success21 :> SuccessCriterion {
				doc /* Procurement and supply chain risk management execution is complementary with the
				 * technical development schedule.
				 */
				:>> number = 21;
			}
		}

		package CDR {
			doc /* Table G-7: Critical Design Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The project has successfully completed the previous planned life-cycle reviews,
				 * and all RFAs and RIDs have been addressed and resolved or a timely closure plan
				 * exists for those remaining open.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* A preliminary CDR agenda, success criteria, and instructions to the review board
				 * have been agreed to by the technical team, project manager, and review chair
				 * prior to the CDR.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* All planned lower level CDRs and peer reviews have been successfully conducted,
				 * and RID/RFA/Action Items have been addressed with the originator or designated
				 * TA.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* A baselined detailed design that can be shown to meet all technical requirements
				 * and performance measures or has waivers.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Other CDR technical work products (as applicable) for hardware, software, and
				 * human system elements have been made available to the cognizant participants
				 * prior to the review: a. Product build-to specifications along with supporting
				 * trade-off analyses and data that are ready to be baselined after review comments
				 * are incorporated. b. Fabrication, assembly, integration, and test plans and
				 * procedures are being developed and are ready to be baselined after review
				 * comments are incorporated. c. Technical data package (e.g., integrated
				 * schematics, spares provisioning list, interface control documents, engineering
				 * analyses, and specifications). d. Status of technical performance related to
				 * margins, TPMs and resolution of the previous review discrepancies addressing
				 * effectiveness of technical achievement and communicating the overall risk to the
				 * project. e. Defined operational limits and constraints. f. Updated technical
				 * resource utilization estimates and margins. g. Acceptance plans that are ready to
				 * be baselined after review comments are incorporated. h. Command and telemetry
				 * list. i. *Updated Verification and Validation Plan. j. Updated integration plans.
				 * k. Preliminary launch site operations plan. l. Preliminary checkout and
				 * activation plan. m. Preliminary disposal plan (including decommissioning or
				 * termination). n. *Updated technology readiness assessment. o. *Updated Technology
				 * Development Plan. p. *Updated risk assessment and mitigation. q. Updated SEMP (or
				 * equivalent program/project documentation). r. Updated Human Systems Integration
				 * approach. (For additional guidance on HSI entrance criteria refer to
				 * NASA/SP-20210010952, NASA Human Systems Integration (HSI) Handbook). s. *Updated
				 * Human Rating Certification Package. t. Updated reliability analyses and
				 * assessments. u. *Updated Life-Cycle Costs and IMS. v. *Updated ILSP. w. *Updated
				 * Project Protection Plan. x. Subsystem-level and preliminary operations safety
				 * analyses that are ready to be baselined after review comments are incorporated.
				 * y. Systems and subsystem certification plans and requirements (as needed) that
				 * are ready to be baselined after review comments are incorporated. a`. *System
				 * safety analysis with associated verifications that is ready to be baselined after
				 * review comments are incorporated. aa. Software criteria and products, per NASA-
				 * HDBK-2203. ab. Received Stage 2 (Experimental) RF system certification signed by
				 * NTIA. ac. Provided measured/as-designed parameter updates to Center/facility
				 * spectrum manager for request for certification of Stage 4 (Operational) spectrum
				 * support no later than 60 days prior to CDR. ad. *Baselined System Security Plan.
				 * ae. Procurement status including Supply Chain Risk Management (SCRM) activities
				 * (e.g., audits and assessments, GIDEP, counterfeit avoidance, surveillance
				 * tailoring). af. List of all single point failures and their effects as well as
				 * rationale for acceptance.
				 */
				:>> number = 6;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The detailed design is expected to meet the requirements with adequate margins.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* Interface control documents are sufficiently mature to proceed with fabrication,
				 * assembly, integration, and test, and plans are in place to manage any open items.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The program/project cost and schedule estimates are credible and within
				 * program/project constraints.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* High confidence exists in the product baseline, and adequate documentation exists
				 * or will exist in a timely manner to allow proceeding with fabrication, assembly,
				 * integration, and test.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* The product verification and product validation requirements and plans are
				 * complete.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* The testing approach is comprehensive, and the planning for system assembly,
				 * integration, test, and launch site and mission operations is sufficient to
				 * progress into the next phase.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Adequate technical and programmatic margins (e.g., mass, power, memory) and
				 * resources exist to complete the development within budget, schedule, and known
				 * risks.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* Risks to safety and mission success are understood and credibly assessed and
				 * plans and resources exist to effectively manage them.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* Safety and mission assurance (e.g., safety, reliability, maintainability, quality
				 * controls, SCRM, QA, and EEE parts) have been adequately addressed in system and
				 * operational designs, and any applicable SMA products (e.g., PRA, system safety
				 * analysis, and failure modes and effects analysis) meet requirements, are at the
				 * appropriate maturity level for this phase of the program/project life cycle, and
				 * indicate that the program/project safety/reliability/system security residual
				 * risks will be at an acceptable level.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* The program/project has demonstrated compliance with applicable NASA and
				 * implementing Center requirements, standards, processes, and procedures.
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* TBD and TBR items are clearly identified with acceptable plans and schedule for
				 * their disposition.
				 */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* Engineering test units, life test units, and/or modeling and simulations have
				 * been developed and tested per plan.
				 */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* Material properties tests are completed along with analyses of loads, stress,
				 * fracture control, contamination generation, and other analyses.
				 */
				:>> number = 13;
			}

			requirement def Success14 :> SuccessCriterion {
				doc /* EEE parts have been selected, and planned testing and delivery will support build
				 * schedules.
				 */
				:>> number = 14;
			}

			requirement def Success15 :> SuccessCriterion {
				doc /* The operational concept has matured, is at a CDR level of detail, and has been
				 * considered in test planning.
				 */
				:>> number = 15;
			}

			requirement def Success16 :> SuccessCriterion {
				doc /* Manufacturability has been adequately included in design. */
				:>> number = 16;
			}

			requirement def Success17 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in NASA-HDBK-2203. */
				:>> number = 17;
			}

			requirement def Success18 :> SuccessCriterion {
				doc /* Human Systems Integration aspects are included in the management and technical
				 * planning following guidelines within NASA/SP-20210010952 and are sufficient to
				 * proceed to the next phase
				 */
				:>> number = 18;
			}

			requirement def Success19 :> SuccessCriterion {
				doc /* Concurrence by the responsible Center spectrum manager that the program/project
				 * has provided requisite RF system data.
				 */
				:>> number = 19;
			}

			requirement def Success20 :> SuccessCriterion {
				doc /* Procurement and supply chain risk management execution is complementary with the
				 * technical development schedule.
				 */
				:>> number = 20;
			}
		}

		package PRR {
			doc /* Table G-8: Production Readiness Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The significant production engineering problems and nonconformances encountered
				 * during development are resolved.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* The design documentation needed to support production is available. */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* The production plans (including but not limited to critical process controls,
				 * control limits, and procedures) and preparation to begin fabrication are
				 * developed.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* The production-enabling products are ready. */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Raw materials are approved and certified. */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Resources are available, have been allocated, and are ready to support end
				 * product production.
				 */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* Updated costs and schedules. */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* Risks have been identified, credibly assessed, and characterized, and mitigation
				 * efforts have been defined.
				 */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* The bill of materials is available and critical parts identified. */
				:>> number = 9;
			}

			requirement def Entrance10 :> EntranceCriterion {
				doc /* Delivery schedules are available. */
				:>> number = 10;
			}

			requirement def Entrance11 :> EntranceCriterion {
				doc /* In-process and end-item inspections and tests have been identified and planned.
				 */
				:>> number = 11;
			}

			requirement def Entrance12 :> EntranceCriterion {
				doc /* Software criteria and products, per NASA-HDBK-2203. */
				:>> number = 12;
			}

			requirement def Entrance13 :> EntranceCriterion {
				doc /* *Spectrum (radio frequency) consideration assessments. */
				:>> number = 13;
			}

			requirement def Entrance14 :> EntranceCriterion {
				doc /* *Updated Human Systems Integration approach. */
				:>> number = 14;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* High confidence exists that the system requirements will be met in the final
				 * production configuration.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* Adequate resources are in place to support production. */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The program/project cost and schedule estimates are credible and within
				 * program/project constraints.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Design-for-manufacturing considerations have been incorporated to ensure ease and
				 * efficiency of production and assembly.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* The product is deemed manufacturable. Evidence is provided that the
				 * program/project is compliant with NASA and Implementing Center requirements,
				 * standards, processes, and procedures.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* TBD and TBR items are clearly identified, with acceptable plans and schedule for
				 * their disposition. Alternate sources for resources have been identified for key
				 * items.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Adequate spares have been planned and budgeted. */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* Required facilities and tools are sufficient for end-product production. */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* Specified special tools and test equipment are available in proper quantities.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* Production and support staff are qualified. */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Drawings and/or production models are approved/certified. */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* Production engineering and planning are sufficiently mature for cost-effective
				 * production.
				 */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* Production processes and methods are consistent with quality requirements and
				 * compliant with occupational health and medical, safety, environmental, and energy
				 * conservation regulations.
				 */
				:>> number = 13;
			}

			requirement def Success14 :> SuccessCriterion {
				doc /* Qualified suppliers are available for materials that are to be procured. */
				:>> number = 14;
			}

			requirement def Success15 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in NASA-HDBK-2203. */
				:>> number = 15;
			}

			requirement def Success16 :> SuccessCriterion {
				doc /* Concurrence by the responsible Center spectrum manager that program/project
				 * complies with RF spectrum policy and regulation.
				 */
				:>> number = 16;
			}

			requirement def Success17 :> SuccessCriterion {
				doc /* PRR plans are mature and results to date indicate high likelihood of supplier
				 * quality control success.
				 */
				:>> number = 17;
			}

			requirement def Success18 :> SuccessCriterion {
				doc /* Production processes and methods are within acceptable risk from threats and
				 * vulnerabilities.
				 */
				:>> number = 18;
			}
		}

		package SIR {
			doc /* Table G-9: System Integration Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The project has successfully completed the previous planned life-cycle reviews,
				 * and all RFAs and RIDs have been addressed and resolved or a timely closure plan
				 * exists for those remaining open.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* A preliminary SIR agenda, success criteria, and instructions to the review board
				 * have been agreed to by the technical team, project manager, and review chair
				 * prior to the SIR.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* The following primary products are ready for review: a. Integration plans
				 * baselined at PDR that have been updated and approved. b. Initial V&V results from
				 * any lower tier products that have been verified.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Status of technical performance related to margins, TPMs, and resolution of the
				 * previous review discrepancies addressing effectiveness of technical achievement
				 * and communicating the overall risk to the project.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Integration procedures have been identified and are scheduled for completion
				 * prior to their need dates.
				 */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* Segments and/or components are on schedule to be available for integration.
				 */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* Mechanical and electrical interface requirements for hardware necessary to start
				 * system integration have been verified in accordance with the interface control
				 * documentation and plans for verification of remaining hardware exist.
				 */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* All functional, unit-level, subsystem, and qualification testing has been
				 * conducted successfully or is on track to be conducted prior to scheduled
				 * integration.
				 */
				:>> number = 9;
			}

			requirement def Entrance10 :> EntranceCriterion {
				doc /* Integration facilities, including clean rooms, ground support equipment, handling
				 * fixtures, overhead cranes, and electrical test equipment, and their associated
				 * quality controls are ready or will be available when required.
				 */
				:>> number = 10;
			}

			requirement def Entrance11 :> EntranceCriterion {
				doc /* Support personnel have been trained. */
				:>> number = 11;
			}

			requirement def Entrance12 :> EntranceCriterion {
				doc /* Handling and safety requirements have been documented. */
				:>> number = 12;
			}

			requirement def Entrance13 :> EntranceCriterion {
				doc /* All known system discrepancies have been identified, dispositioned, and are on
				 * schedule for closure.
				 */
				:>> number = 13;
			}

			requirement def Entrance14 :> EntranceCriterion {
				doc /* The quality control organization is ready to support integration effort. */
				:>> number = 14;
			}

			requirement def Entrance15 :> EntranceCriterion {
				doc /* Other SIR technical products (as applicable) for hardware, software, and human
				 * system elements have been made available to the cognizant participants prior to
				 * the review: a. *Updated Life-Cycle Costs and IMS. b. *Updated design solution
				 * definition. c. Updated interface definition(s). d. *Updated verification and
				 * validation plans. e. Final transportation criteria and instructions. f.
				 * *Preliminary mission operations plans. g. Preliminary decommissioning plans. h.
				 * Preliminary disposal plans. i. Software criteria and products, per NASA-
				 * HDBK-2203. j. Procurement status including Supply Chain Risk Management (SCRM)
				 * activities (e.g., audits and assessments, GIDEP, counterfeit avoidance).
				 */
				:>> number = 15;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Integration plans and procedures are on track for completion and approval to
				 * support system integration.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* Previous component, subsystem, and system test results form a satisfactory basis
				 * for proceeding to integration.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The program/project cost and schedule estimates are credible with adequate
				 * margins and within program/project constraints.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Risks are identified and accepted by program/project leadership, as required.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* The program/project has demonstrated compliance with applicable NASA and
				 * implementing Center requirements, standards, processes, and procedures.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* TBD and TBR items are clearly identified with acceptable plans and schedule for
				 * their dispositions.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* The integration procedures and workflow have been clearly defined and documented
				 * or are on schedule to be clearly defined and documented prior to their need date.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* The review of the integration plans, as well as the procedures, environment, and
				 * configuration of the items to be integrated, provides a reasonable expectation
				 * that the integration will proceed successfully.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* All training necessary to properly integrate the system has been performed.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in NASA-HDBK-2203. */
				:>> number = 10;
			}
		}

		package TRR {
			doc /* Table G-10: Test Readiness Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* A preliminary TRR agenda, success criteria, and instructions to the review team
				 * have been agreed to by the technical team, project manager, and review chair
				 * prior to the TRR.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* The objectives of the testing have been clearly defined and documented. */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* Approved test plans, test procedures, test environment, and configuration of the
				 * test item(s) that support test objectives are available.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* All test interfaces have been placed under configuration control or have been
				 * defined in accordance with an agreed to plan, and version description document(s)
				 * for both test and support systems have been made available to TRR participants
				 * prior to the review.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* All known system discrepancies have been identified and dispositioned in
				 * accordance with an agreed-upon plan.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* All required test resourcesâ?"people (including a designated test director),
				 * facilities, test articles, test instrumentation, and other test-enabling
				 * productsâ?"have been identified and are available to support required tests.
				 */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* Roles and responsibilities of all test participants are defined and agreed to.
				 */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* Test safety planning has been accomplished, and all personnel have been trained.
				 */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* Spectrum (radio frequency) considerations addressed. */
				:>> number = 9;
			}

			requirement def Entrance10 :> EntranceCriterion {
				doc /* As-built hardware and software documentation defining the configuration of the
				 * item under test are released and under configuration control.
				 */
				:>> number = 10;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Adequate test plans are completed and approved for the system under test. */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* Adequate identification and coordination of required test resources are
				 * completed.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The program/project has demonstrated compliance with applicable NASA and
				 * implementing Center requirements, standards, processes, and procedures.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* TBD and TBR items are clearly identified with acceptable plans and schedule for
				 * their disposition.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* Risks have been identified, credibly assessed, and appropriately mitigated.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Residual risk is accepted by program/project leadership as required. */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Plans to capture any lessons learned from the test program are documented.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* The objectives of the testing have been clearly defined and documented, and the
				 * review of all the test plans, as well as the procedures, environment, and
				 * configuration of the test item, provides a reasonable expectation that the
				 * objectives will be met.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* The test cases have been analyzed and are consistent with the test plans and
				 * objectives.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* Test personnel have received appropriate training in test operation and health
				 * and medical safety procedures.
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* *Concurrence by the responsible Center spectrum manager that all tests are
				 * performed. in accordance with spectrum policy and regulation.
				 */
				:>> number = 11;
			}
		}

		package SAR {
			doc /* Table G-11: System Acceptance Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The project has successfully completed the previous planned life-cycle reviews,
				 * RFA/RIDs have been closed, and plans to complete open work are defined.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* A preliminary SAR agenda, success criteria, and instructions to the review team
				 * have been agreed to by the technical team, project manager, and review chair
				 * prior to the review.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* The following SAR technical products have been made available to the cognizant
				 * participants prior to the review: a. Results of the SARs conducted at the major
				 * suppliers. b. Product verification results. c. Product validation results. d.
				 * Documentation that the delivered system complies with the established acceptance
				 * criteria. e. Documentation that the system will perform properly in the expected
				 * operational environment. f. Technical data package that has been updated to
				 * include all test results. g. Final Certification Package. h. Baselined as-built
				 * hardware and software documentation. i. Updated risk assessment and mitigation.
				 * j. Required safety, shipping, handling, checkout, and operational plans and
				 * procedures. k. Software criteria and products, per NASA-HDBK-2203. l. *Received
				 * Stage 4 (Operational) system certification signed by NTIA. m. Completed planning
				 * for sustaining the system. n. Updated list of all single point failures and their
				 * effects.
				 */
				:>> number = 3;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Required tests and analyses are complete and indicate that the system will
				 * perform properly in the expected operational environment.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* Risks are identified and mitigated to acceptable levels. */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* System meets the established acceptance criteria. */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* TBD and TBR items are resolved. */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* Acceptance data package is complete and reflects the delivered system. */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* All applicable lessons learned for organizational improvement and system
				 * operations are captured.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in NASA-HDBK-2203. */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* *Concurrence by the responsible Center spectrum manager that the Stage 4
				 * (Operational) system certification has been obtained and the system is compliant
				 * with spectrum policy and regulation.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* The system hardware, software, documentation, and associated products are
				 * complete and ready for acceptance.
				 */
				:>> number = 9;
			}
		}

		package ORR {
			doc /* Table G-12: Operational Readiness Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* All planned ground-based testing has been completed. */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* Test failures and anomalies from verification and validation testing have been
				 * resolved, and the results/mitigations/work-arounds have been incorporated into
				 * supporting and enabling operational products.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* All operational supporting and enabling products (e.g., facilities, equipment,
				 * documents, software tools, databases) that are necessary for nominal and
				 * contingency operations have been tested and delivered/installed at the site(s)
				 * necessary to support operations.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Operations documentation (e.g., handbook, procedures) has been written, verified,
				 * and approved.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Users/operators have been trained on the correct operation of the system. */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* Operational contingency planning has been completed, and operations personnel
				 * have been trained on their use.
				 */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* The following primary products are ready for review: a. Preliminary V&V results.
				 * b. Baseline decommissioning plan. c. Baseline disposal plans.
				 */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* Other ORR technical products have been made available to the cognizant
				 * participants prior to the review: a. *Updated cost and schedule. b. *Updated
				 * Project Protection Plan. c. Updated as-built hardware and software documentation.
				 * d. Baselined operations plans. e. Updated operational procedures. f. Preliminary
				 * certification for flight/use. g. *Updated Human Rating Certification Package. h.
				 * Software criteria and products, per NASA-HDBK-2203.
				 */
				:>> number = 9;
			}

			requirement def Entrance10 :> EntranceCriterion {
				doc /* Received Stage 4 (Operational) system certification signed by NTIA. */
				:>> number = 10;
			}

			requirement def Entrance11 :> EntranceCriterion {
				doc /* All requisite radio frequency authorizations are in place. */
				:>> number = 11;
			}

			requirement def Entrance12 :> EntranceCriterion {
				doc /* Updated list of all single point failures (SPF) and their effects including
				 * rationale for acceptance of any new SPFs.
				 */
				:>> number = 12;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The system, including all enabling products, is determined to be ready to be
				 * placed in an operational status.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* All applicable lessons learned for organizational improvement and systems
				 * operations have been captured.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* All waivers and anomalies have been closed. */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Systems hardware, software, personnel, tools, supporting infrastructure, and
				 * procedures are in place to support operations.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* Operations plans and schedules are consistent with mission objectives. */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Mission risks have been identified, planned mitigations are adequate, and
				 * residual risks are accepted by the program/project manager.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Testing is consistent with the expected operational environment. */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* The program/project cost and schedule estimates are credible and within
				 * program/project constraints.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* The program/project has demonstrated compliance with applicable NASA and
				 * implementing Center requirements, standards, processes, and procedures.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* TBD and TBR items are resolved. */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in NASA-HDBK-2203. */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* Concurrence by the responsible Center spectrum manager that all necessary
				 * spectrum certification(s) and authorization(s) have been obtained.
				 */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* An operational Human Systems Integration capability has been established and HSI
				 * planning is in place for the remaining life-cycle phases.
				 */
				:>> number = 13;
			}
		}

		package MRR_FRR {
			doc /* Table G-13: Mission Readiness Review/Flight Readiness Review entrance and success
			 * criteria.
			 */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The system and support elements are ready and have been properly configured for
				 * flight/mission operations.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* System and support element interfaces have been demonstrated to function as
				 * expected.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* The system state supports a launch â??goâ?? decision based on the established
				 * go/no-go criteria.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Failures and anomalies from previously completed flights, tests, and reviews have
				 * been resolved, and the results/mitigations/work-arounds have been incorporated
				 * into supporting and enabling operational products.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* The following primary products are ready for review: a. Final certification for
				 * flight/use. b. Baselined V&V results.
				 */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* Other MRR/FRR technical products have been made available to the cognizant
				 * participants prior to the review: a. *Updated cost. b. *Updated schedule. c.
				 * Updated as-built hardware and software documentation. d. Updated operations
				 * procedures. e. Updated decommissioning plan. f. Updated disposal plan g. Software
				 * criteria and products, per NASA-HDBK-2203.
				 */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* Received Stage 4 (Operational) system certification signed by NTIA. */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* All requisite spectrum (radio frequency) authorizations are in place. */
				:>> number = 9;
			}

			requirement def Entrance10 :> EntranceCriterion {
				doc /* Updated list of all single point failures and their effects. */
				:>> number = 10;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The flight vehicle/system is ready for flight/mission operations. */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The hardware is deemed acceptably safe for flight/mission operations. */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* Certification that flight operations can safely proceed with acceptable risk has
				 * been achieved.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Flight and ground software elements are ready to support launch and flight
				 * operations.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* Interfaces have been checked and demonstrated to be functional. */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* The program/project has demonstrated compliance with applicable NASA and
				 * implementing Center requirements, standards, processes, and procedures.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* TBD and TBR items are resolved. */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* Open items and waivers have been examined and residual risk from these is deemed
				 * to be acceptable.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* The flight and recovery environmental factors are within constraints. */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* All open safety and mission risk items have been addressed, and the residual risk
				 * is deemed acceptable.
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Supporting organizations are ready to support flight/mission operations. */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* Software components meet the success criteria defined in NASA-HDBK-2203. */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* Responsible Center spectrum manager(s) concur that all necessary spectrum
				 * certification(s) and authorization(s) have been obtained.
				 */
				:>> number = 13;
			}
		}

		package PLAR {
			doc /* Table G-14: Post-Launch Assessment Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The launch and early operations performance, including (when appropriate) the
				 * early propulsive maneuver results, are available.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* The observed spacecraft and science instrument performance, including instrument
				 * calibration plans and status, are available.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* The launch vehicle performance assessment and mission implications, including
				 * launch sequence assessment and launch operations experience with lessons learned,
				 * are completed.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* The mission operations and ground data system experience, including tracking and
				 * data acquisition support and spacecraft telemetry support and spacecraft
				 * telemetry data analysis, is available.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* The mission operations organization, including status of staffing, facilities,
				 * tools, and mission software (e.g., spacecraft analysis and sequencing), is
				 * available.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* In-flight anomalies and the responsive actions taken, including any autonomous
				 * fault protection actions taken by the spacecraft or any unexplained spacecraft
				 * telemetry, including alarms, are documented.
				 */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* The need for significant changes to the system (e.g., hardware, software, or
				 * interfaces), support systems, operations (e.g., schedules, processes and
				 * procedures), and staffing has been documented.
				 */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* Documentation is updated, including any updates originating from the early
				 * operations experience.
				 */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* Plans for post-launch development have been addressed. */
				:>> number = 9;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The observed spacecraft and science payload performance agrees with prediction,
				 * or if not, is adequately understood so that future behavior can be predicted with
				 * confidence.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* All anomalies have been adequately documented and their impact on operations
				 * assessed. Further, anomalies impacting spacecraft health and medical, safety, or
				 * critical flight operations have been properly dispositioned.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The mission operations capabilities, including staffing and plans, are adequate
				 * to accommodate the actual flight performance.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Open items, if any, on operations identified as part of the ORR have identified
				 * as part of the ORR have been satisfactorily dispositioned.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* *Concurrence by the responsible Center spectrum manager that the system is
				 * compliant with spectrum policy and regulation.
				 */
				:>> number = 5;
			}
		}

		package CERR {
			doc /* Table G-15: Critical Event Readiness Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* Critical event/activity requirements and constraints have been identified,
				 * including spectrum considerations.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* Critical event/activity design and implementation are complete. */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* Critical event/activity testing is complete. */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Critical event/activity operations planning, including contingencies, is
				 * complete.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Operations personnel training for the critical event/activity has been conducted.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Critical event/activity sequence verification and validation is complete. */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* Flight system is healthy and capable of performing the critical event/activity.
				 */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* Flight failures and anomalies from critical event/activity testing have been
				 * resolved, and the results/mitigations/work-arounds have been incorporated into
				 * supporting and enabling operational products.
				 */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* The following technical products have been made available to the cognizant
				 * participants prior to the review: a. Final certification for critical event
				 * readiness. b. Updated operations procedures.
				 */
				:>> number = 9;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The critical activity design complies with requirements. The preparation for the
				 * critical activity, including the verification and validation, is thorough.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The project (including all the systems, supporting services, and documentation)
				 * is ready to support the activity.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The requirements for the successful execution of the critical event(s) are
				 * complete and understood and have flowed down to the appropriate levels for
				 * implementation.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Any TBD and TBR items related to the critical event have been resolved. */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* All open risk items related to the critical event have been addressed, and the
				 * residual risk is deemed acceptable.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* *Concurrence by the responsible Center spectrum manager that the system is
				 * compliant with spectrum policy and regulation.
				 */
				:>> number = 6;
			}
		}

		package PFAR {
			doc /* Table G-16: Post-Flight Assessment Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* All anomalies that occurred during the mission, as well as during preflight
				 * testing, countdown, and ascent, are dispositioned.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* All flight and post-flight documentation applicable to future flights of the
				 * spacecraft or the design is available.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* All planned activities to be performed post-flight have been completed. */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Problem reports, corrective action requests, and post-flight anomaly records are
				 * completed. Include spectrum (radio frequency) interference or other related
				 * factors during assessment.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* All post-flight hardware and flight performance data evaluation reports are
				 * completed.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Plans for retaining assessment documentation and imaging have been made. */
				:>> number = 6;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Formal final report documenting flight performance and recommendations for future
				 * missions is complete and adequate.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* All anomalies have been adequately documented and dispositioned. */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* The impact of anomalies on future flight operations has been assessed and
				 * documented.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Reports and other documentation have been retained for performance comparison and
				 * trending.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* Responsible Center spectrum manager was notified of any RF spectrum interference
				 * issues.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Recommendations for updates to the system design, test and operations procedures,
				 * or safety inspections have been identified and a credible plan exists to
				 * incorporate the changes.
				 */
				:>> number = 6;
			}
		}

		package DR {
			doc /* Table G-17: Decommissioning Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The requirements associated with decommissioning are defined. */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* Plans are in place for decommissioning and any other removal from service
				 * activities.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* Resources are in place to support and implement decommissioning. */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Health and medical, safety, environmental, and any other constraints have been
				 * identified.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Current system capabilities relating to decommissioning are understood. to
				 * decommissioning are understood.
				 */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* Off-nominal operations, all contributing events, conditions, and changes to the
				 * originally expected baseline have been considered and assessed.
				 */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* The following primary product is ready for review: a. Updated decommissioning
				 * plan.
				 */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* Other DR technical products have been made available to the cognizant
				 * participants prior to the review: a. *Updated cost. b. Updated schedule. c.
				 * *Updated disposal plan.
				 */
				:>> number = 9;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The rationale for decommissioning is documented. */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The decommissioning plan is complete, meets requirements, is approved by
				 * appropriate management, and is compliant with applicable Agency safety,
				 * environmental, and health regulations.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* Operations plans for decommissioning, including contingencies, are complete and
				 * approved.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Adequate resources (schedule, budget, and staffing) have been identified and are
				 * available to identified and are available to successfully complete all
				 * decommissioning activities.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* All required support systems for decommissioning are available. */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* All personnel have been properly trained for the nominal and contingency
				 * decommissioning procedures.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Safety, health, and environmental hazards have been identified, and controls have
				 * been verified.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* Risks associated with the decommissioning have been identified and adequately
				 * mitigated.
				 */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* Residual risks have been accepted by the required management. */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* Any TBD and TBR items are clearly identified with acceptable plans and schedule
				 * for their disposition.
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Plans for archival and subsequent analysis of mission data have been defined and
				 * approved, and arrangements have been finalized for the execution of such plans.
				 */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* Plans for the capture and dissemination of appropriate lessons learned during the
				 * project life cycle have been defined and approved.
				 */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* Plans for transition of personnel have been defined and approved. */
				:>> number = 13;
			}

			requirement def Success14 :> SuccessCriterion {
				doc /* Concurrence by the responsible Center spectrum manager that the decommissioning
				 * plans are compliant with spectrum policy and regulation.
				 */
				:>> number = 14;
			}
		}

		package DRR {
			doc /* Table G-18: Disposal Readiness Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* Requirements associated with disposal are defined. */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* Plans are in place for disposal and any other removal from service activities.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* Resources are in place to support disposal. */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Safety, environmental, health, and any other constraints are described. */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Current system capabilities related to disposal are described and understood.
				 */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Off-nominal operations, all contributing events, conditions, and changes to the
				 * originally expected baseline have been considered and assessed.
				 */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* *Updated cost. */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* Updated schedule. */
				:>> number = 8;
			}

			requirement def Entrance9 :> EntranceCriterion {
				doc /* The following primary product is ready for review: a. Updated disposal plan.
				 */
				:>> number = 9;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* The rationale for disposal is documented. */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The disposal plan is complete, meets requirements, is approved by appropriate
				 * management, and is compliant with applicable Agency safety, environmental, and
				 * health regulations.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* Operations plans for disposal, including contingencies, are complete and
				 * approved.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* All required support systems for disposal are available. */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* All personnel have been properly trained for the nominal and contingency disposal
				 * procedures.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Safety, health, and environmental hazards have been identified, and controls have
				 * been verified.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Risks associated with the disposal have been identified and adequately mitigated.
				 */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* Residual risks have been accepted by the required management. */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* If hardware is to be recovered from orbit: a. Return site activity plans have
				 * been defined and approved. b. Required facilities are available and meet
				 * requirements, including those for contamination control, if needed. c.
				 * Transportation plans are defined and approved. d. Shipping containers and
				 * handling equipment, as well as contamination and environmental control and
				 * monitoring devices, are available.
				 */
				:>> number = 9;
			}

			requirement def Success10 :> SuccessCriterion {
				doc /* Plans for disposition of mission-owned assets (i.e., hardware, software,
				 * facilities and data) have been defined and approved.
				 */
				:>> number = 10;
			}

			requirement def Success11 :> SuccessCriterion {
				doc /* Adequate resources (schedule, budget, and staffing) have been identified and are
				 * available to successfully complete all disposal activities.
				 */
				:>> number = 11;
			}

			requirement def Success12 :> SuccessCriterion {
				doc /* All mission and project data and documentation has been archived per disposal
				 * plan.
				 */
				:>> number = 12;
			}

			requirement def Success13 :> SuccessCriterion {
				doc /* TBD and TBR items related to system disposal have all been dispositioned. */
				:>> number = 13;
			}

			requirement def Success14 :> SuccessCriterion {
				doc /* Concurrence by the responsible Center spectrum manager that the disposal plans
				 * are compliant with spectrum policy and regulation.
				 */
				:>> number = 14;
			}
		}

		package PeerReview {
			doc /* Table G-19: Peer Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The product to be reviewed (e.g., document, process, model, design details) has
				 * been identified and made available to the review team.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* Peer reviewers independent from the project have been selected for their
				 * technical background related to the product being reviewed.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* A preliminary agenda, success criteria, and instructions to the review team have
				 * been agreed to by the technical team and project manager.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Rules have been established to ensure consistency among the team members involved
				 * in the peer review process.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* *Spectrum (radio frequency) considerations addressed. */
				:>> number = 5;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Peer review has thoroughly evaluated the technical integrity and quality of the
				 * product.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* Any defects have been identified and characterized. */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* Results of the peer review are communicated to the appropriate project personnel.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Spectrum-related aspects have been concurred to by the responsible Center
				 * spectrum manager.
				 */
				:>> number = 4;
			}
		}

		package PIR_PSR {
			doc /* Table G-20: Program Implementation Review/Program Status Review entrance and success
			 * criteria.
			 */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* A preliminary PIR agenda, success criteria, and instructions to the review team
				 * have been agreed to by the technical team, project manager, and review chair
				 * prior to the review.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* The current status of the overall technical effort is available and ready to be
				 * reviewed.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* Programmatic products are ready for review at the maturity levels stated in the
				 * governing program/project management NPR.
				 */
				:>> number = 3;
			}

			requirement def Entrance4 :> EntranceCriterion {
				doc /* Current actual and estimated costs are available and compared to the expected
				 * plan.
				 */
				:>> number = 4;
			}

			requirement def Entrance5 :> EntranceCriterion {
				doc /* Current schedule is available showing remaining work planned. */
				:>> number = 5;
			}

			requirement def Entrance6 :> EntranceCriterion {
				doc /* Trending of the selected Technical Performance Parameters relevant to the current
				 * Program phase is available.
				 */
				:>> number = 6;
			}

			requirement def Entrance7 :> EntranceCriterion {
				doc /* Updated technical plans are available. */
				:>> number = 7;
			}

			requirement def Entrance8 :> EntranceCriterion {
				doc /* *Spectrum (radio frequency) considerations addressed. */
				:>> number = 8;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Program still meets Agency needs and should continue. */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* The program cost and schedule estimates are credible and within program
				 * constraints.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* Risks are identified and accepted by program/project leadership, as required.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Technical trends are within acceptable bounds. */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* Adequate progress has been made relative to plans, including the technology
				 * readiness levels.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* For technology development programs, technologies have been identified that are
				 * ready to be transitioned to another project or to an organization outside the
				 * Agency.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Spectrum-related aspects have been concurred to by the responsible Center
				 * spectrum manager.
				 */
				:>> number = 7;
			}
		}

		package DCR {
			doc /* Table G-21: Design Certification Review entrance and success criteria. */

			requirement def Entrance1 :> EntranceCriterion {
				doc /* The project has successfully completed the previous planned life-cycle reviews,
				 * RFA/RIDs have been closed, and plans to complete open work are defined.
				 */
				:>> number = 1;
			}

			requirement def Entrance2 :> EntranceCriterion {
				doc /* A preliminary DCR agenda, success criteria, and instructions to the review team
				 * have been agreed to by the technical team, project manager, and review chair
				 * prior to the review.
				 */
				:>> number = 2;
			}

			requirement def Entrance3 :> EntranceCriterion {
				doc /* The following DCR technical products have been made available to the cognizant
				 * participants prior to the review: a. Updated Verification and Validation Plan. b.
				 * As-run qualification test procedures, configurations, test environments, and test
				 * results. c. Product verification results. d. Product validation results. e.
				 * Documentation that the system will perform properly in the design environments.
				 * f. Final design certification package. g. Safety products (e.g., Failure Mode and
				 * Effects Analysis/Critical Items Lists (FMEA/CILs), Failure Mode, (FMEA/CILs),
				 * Failure Mode, Effects, and Criticality Analysis (FMECA), Safety, Hazard Reports).
				 * h. All operating, production or fabrication, and maintenance constraints are
				 * documented. i. Updated risk assessment and mitigation. j. Waivers/deviations
				 * affecting the qualification articles, procedures, or environments.
				 */
				:>> number = 3;
			}

			requirement def Success1 :> SuccessCriterion {
				doc /* Qualification tests, configurations, and test environments demonstrate the system
				 * can meet functional and performance requirements across all applicable flight
				 * envelopes, configurations, and environments.
				 */
				:>> number = 1;
			}

			requirement def Success2 :> SuccessCriterion {
				doc /* Required tests and analyses are complete and indicate that the system will
				 * perform properly in the expected design environments.
				 */
				:>> number = 2;
			}

			requirement def Success3 :> SuccessCriterion {
				doc /* Design certification data package is complete and reflects the as-certified
				 * system.
				 */
				:>> number = 3;
			}

			requirement def Success4 :> SuccessCriterion {
				doc /* Waivers/deviations and non-conformance affecting the qualification test articles,
				 * procedures, or environments have been approved.
				 */
				:>> number = 4;
			}

			requirement def Success5 :> SuccessCriterion {
				doc /* Design mitigations have been appropriately implemented in response to safety
				 * products (e.g., FEMA/CILs, FMECA, Safety, and Hazard Reports) and indicate
				 * residual safety and mission success risks are acceptable for all intended uses of
				 * the system.
				 */
				:>> number = 5;
			}

			requirement def Success6 :> SuccessCriterion {
				doc /* Operating, production or fabrication, and maintenance constraints demonstrate a
				 * viable path to producing the system per the design. the design.
				 */
				:>> number = 6;
			}

			requirement def Success7 :> SuccessCriterion {
				doc /* Risks are known and manageable. */
				:>> number = 7;
			}

			requirement def Success8 :> SuccessCriterion {
				doc /* TBD and TBR items are resolved. */
				:>> number = 8;
			}

			requirement def Success9 :> SuccessCriterion {
				doc /* *Concurrence by the responsible Center spectrum manager that all tests are
				 * performed in accordance with spectrum policy and regulation.
				 */
				:>> number = 9;
			}
		}
	}

	/* Semantic metadata: the user-defined keywords of the library */

	metadata def <technicalProcess> TechnicalProcessMetadata :> SemanticMetadata {
		doc /* The `#technicalProcess` keyword: classifies an action definition or action usage as a
		 * technicalProcesse by applying the definition and the `technicalProcesses` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = technicalProcesses meta SysML::Usage;
	}

	metadata def <stakeholderExpectationsDefinition> StakeholderExpectationsDefinitionMetadata :> SemanticMetadata {
		doc /* The `#stakeholderExpectationsDefinition` keyword: classifies an action definition or
		 * action usage as a stakeholderExpectationsDefinition by applying the definition and the
		 * `stakeholderExpectationsDefinitions` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = stakeholderExpectationsDefinitions meta SysML::Usage;
	}

	metadata def <technicalRequirementsDefinition> TechnicalRequirementsDefinitionMetadata :> SemanticMetadata {
		doc /* The `#technicalRequirementsDefinition` keyword: classifies an action definition or action
		 * usage as a technicalRequirementsDefinition by applying the definition and the
		 * `technicalRequirementsDefinitions` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = technicalRequirementsDefinitions meta SysML::Usage;
	}

	metadata def <logicalDecomposition> LogicalDecompositionMetadata :> SemanticMetadata {
		doc /* The `#logicalDecomposition` keyword: classifies an action definition or action usage as a
		 * logicalDecomposition by applying the definition and the `logicalDecompositions` base
		 * usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = logicalDecompositions meta SysML::Usage;
	}

	metadata def <designSolutionDefinition> DesignSolutionDefinitionMetadata :> SemanticMetadata {
		doc /* The `#designSolutionDefinition` keyword: classifies an action definition or action usage
		 * as a designSolutionDefinition by applying the definition and the
		 * `designSolutionDefinitions` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = designSolutionDefinitions meta SysML::Usage;
	}

	metadata def <productImplementation> ProductImplementationMetadata :> SemanticMetadata {
		doc /* The `#productImplementation` keyword: classifies an action definition or action usage as
		 * a productImplementation by applying the definition and the `productImplementations` base
		 * usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = productImplementations meta SysML::Usage;
	}

	metadata def <productIntegration> ProductIntegrationMetadata :> SemanticMetadata {
		doc /* The `#productIntegration` keyword: classifies an action definition or action usage as a
		 * productIntegration by applying the definition and the `productIntegrations` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = productIntegrations meta SysML::Usage;
	}

	metadata def <productVerification> ProductVerificationMetadata :> SemanticMetadata {
		doc /* The `#productVerification` keyword: classifies an action definition or action usage as a
		 * productVerification by applying the definition and the `productVerifications` base usage
		 * at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = productVerifications meta SysML::Usage;
	}

	metadata def <productValidation> ProductValidationMetadata :> SemanticMetadata {
		doc /* The `#productValidation` keyword: classifies an action definition or action usage as a
		 * productValidation by applying the definition and the `productValidations` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = productValidations meta SysML::Usage;
	}

	metadata def <productTransition> ProductTransitionMetadata :> SemanticMetadata {
		doc /* The `#productTransition` keyword: classifies an action definition or action usage as a
		 * productTransition by applying the definition and the `productTransitions` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = productTransitions meta SysML::Usage;
	}

	metadata def <technicalPlanning> TechnicalPlanningMetadata :> SemanticMetadata {
		doc /* The `#technicalPlanning` keyword: classifies an action definition or action usage as a
		 * technicalPlanning by applying the definition and the `technicalPlannings` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = technicalPlannings meta SysML::Usage;
	}

	metadata def <requirementsManagement> RequirementsManagementMetadata :> SemanticMetadata {
		doc /* The `#requirementsManagement` keyword: classifies an action definition or action usage as
		 * a requirementsManagement by applying the definition and the `requirementsManagements`
		 * base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = requirementsManagements meta SysML::Usage;
	}

	metadata def <interfaceManagement> InterfaceManagementMetadata :> SemanticMetadata {
		doc /* The `#interfaceManagement` keyword: classifies an action definition or action usage as a
		 * interfaceManagement by applying the definition and the `interfaceManagements` base usage
		 * at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = interfaceManagements meta SysML::Usage;
	}

	metadata def <technicalRiskManagement> TechnicalRiskManagementMetadata :> SemanticMetadata {
		doc /* The `#technicalRiskManagement` keyword: classifies an action definition or action usage
		 * as a technicalRiskManagement by applying the definition and the
		 * `technicalRiskManagements` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = technicalRiskManagements meta SysML::Usage;
	}

	metadata def <configurationManagement> ConfigurationManagementMetadata :> SemanticMetadata {
		doc /* The `#configurationManagement` keyword: classifies an action definition or action usage
		 * as a configurationManagement by applying the definition and the
		 * `configurationManagements` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = configurationManagements meta SysML::Usage;
	}

	metadata def <technicalDataManagement> TechnicalDataManagementMetadata :> SemanticMetadata {
		doc /* The `#technicalDataManagement` keyword: classifies an action definition or action usage
		 * as a technicalDataManagement by applying the definition and the
		 * `technicalDataManagements` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = technicalDataManagements meta SysML::Usage;
	}

	metadata def <technicalAssessment> TechnicalAssessmentMetadata :> SemanticMetadata {
		doc /* The `#technicalAssessment` keyword: classifies an action definition or action usage as a
		 * technicalAssessment by applying the definition and the `technicalAssessments` base usage
		 * at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = technicalAssessments meta SysML::Usage;
	}

	metadata def <decisionAnalysis> DecisionAnalysisMetadata :> SemanticMetadata {
		doc /* The `#decisionAnalysis` keyword: classifies an action definition or action usage as a
		 * decisionAnalysis by applying the definition and the `decisionAnalysiss` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = decisionAnalysiss meta SysML::Usage;
	}

	metadata def <keyDecisionPoint> KeyDecisionPointMetadata :> SemanticMetadata {
		doc /* The `#keyDecisionPoint` keyword: classifies an occurrence definition or occurrence usage
		 * as a keyDecisionPoint by applying the definition and the `keyDecisionPoints` base usage
		 * at once.
		 */
		:> annotatedElement : SysML::OccurrenceDefinition;
		:> annotatedElement : SysML::OccurrenceUsage;
		:>> baseType = keyDecisionPoints meta SysML::Usage;
	}

	metadata def <technicalReview> TechnicalReviewMetadata :> SemanticMetadata {
		doc /* The `#technicalReview` keyword: classifies an action definition or action usage as a
		 * technicalReview by applying the definition and the `technicalReviews` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = technicalReviews meta SysML::Usage;
	}

	metadata def <lifeCycleReview> LifeCycleReviewMetadata :> SemanticMetadata {
		doc /* The `#lifeCycleReview` keyword: classifies an action definition or action usage as a
		 * lifeCycleReview by applying the definition and the `lifeCycleReviews` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = lifeCycleReviews meta SysML::Usage;
	}

	metadata def <peerReview> PeerReviewMetadata :> SemanticMetadata {
		doc /* The `#peerReview` keyword: classifies an action definition or action usage as a
		 * peerReview by applying the definition and the `peerReviews` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = peerReviews meta SysML::Usage;
	}

	metadata def <entranceCriterion> EntranceCriterionMetadata :> SemanticMetadata {
		doc /* The `#entranceCriterion` keyword: classifies an requirement definition or requirement
		 * usage as a reviewEntranceCriteria by applying the definition and the
		 * `reviewEntranceCriteria` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = reviewEntranceCriteria meta SysML::Usage;
	}

	metadata def <successCriterion> SuccessCriterionMetadata :> SemanticMetadata {
		doc /* The `#successCriterion` keyword: classifies an requirement definition or requirement
		 * usage as a reviewSuccessCriteria by applying the definition and the
		 * `reviewSuccessCriteria` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = reviewSuccessCriteria meta SysML::Usage;
	}

	metadata def <reviewItem> ReviewItemMetadata :> SemanticMetadata {
		doc /* The `#reviewItem` keyword: classifies an item definition or item usage as a reviewItem by
		 * applying the definition and the `reviewItems` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = reviewItems meta SysML::Usage;
	}

	metadata def <workProduct> WorkProductMetadata :> SemanticMetadata {
		doc /* The `#workProduct` keyword: classifies an item definition or item usage as a workProduct
		 * by applying the definition and the `workProducts` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = workProducts meta SysML::Usage;
	}

	metadata def <stakeholderIdentificationAndExpectationsDefinition> StakeholderIdentificationAndExpectationsDefinitionMetadata :> SemanticMetadata {
		doc /* The `#stakeholderIdentificationAndExpectationsDefinition` keyword: classifies an item
		 * definition or item usage as a stakeholderIdentificationAndExpectationsDefinition by
		 * applying the definition and the `stakeholderIdentificationAndExpectationsDefinitions`
		 * base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = stakeholderIdentificationAndExpectationsDefinitions meta SysML::Usage;
	}

	metadata def <conceptDefinition> ConceptDefinitionMetadata :> SemanticMetadata {
		doc /* The `#conceptDefinition` keyword: classifies an item definition or item usage as a
		 * conceptDefinition by applying the definition and the `conceptDefinitions` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = conceptDefinitions meta SysML::Usage;
	}

	metadata def <measureOfEffectivenessDefinition> MeasureOfEffectivenessDefinitionMetadata :> SemanticMetadata {
		doc /* The `#measureOfEffectivenessDefinition` keyword: classifies an item definition or item
		 * usage as a measureOfEffectivenessDefinition by applying the definition and the
		 * `measureOfEffectivenessDefinitions` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = measureOfEffectivenessDefinitions meta SysML::Usage;
	}

	metadata def <costAndScheduleForTechnicalImplementation> CostAndScheduleForTechnicalImplementationMetadata :> SemanticMetadata {
		doc /* The `#costAndScheduleForTechnicalImplementation` keyword: classifies an item definition
		 * or item usage as a costAndScheduleForTechnicalImplementation by applying the definition
		 * and the `costAndScheduleForTechnicalImplementations` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = costAndScheduleForTechnicalImplementations meta SysML::Usage;
	}

	metadata def <requirementSet> RequirementSetMetadata :> SemanticMetadata {
		doc /* The `#requirementSet` keyword: classifies an item definition or item usage as a
		 * requirementSet by applying the definition and the `requirementSets` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = requirementSets meta SysML::Usage;
	}

	metadata def <technicalPerformanceMeasureDefinition> TechnicalPerformanceMeasureDefinitionMetadata :> SemanticMetadata {
		doc /* The `#technicalPerformanceMeasureDefinition` keyword: classifies an item definition or
		 * item usage as a technicalPerformanceMeasureDefinition by applying the definition and the
		 * `technicalPerformanceMeasureDefinitions` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = technicalPerformanceMeasureDefinitions meta SysML::Usage;
	}

	metadata def <humanSystemsIntegrationApproach> HumanSystemsIntegrationApproachMetadata :> SemanticMetadata {
		doc /* The `#humanSystemsIntegrationApproach` keyword: classifies an item definition or item
		 * usage as a humanSystemsIntegrationApproach by applying the definition and the
		 * `humanSystemsIntegrationApproachs` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = humanSystemsIntegrationApproachs meta SysML::Usage;
	}

	metadata def <architectureDefinition> ArchitectureDefinitionMetadata :> SemanticMetadata {
		doc /* The `#architectureDefinition` keyword: classifies an item definition or item usage as a
		 * architectureDefinition by applying the definition and the `architectureDefinitions` base
		 * usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = architectureDefinitions meta SysML::Usage;
	}

	metadata def <allocationOfRequirementsToNextLowerLevel> AllocationOfRequirementsToNextLowerLevelMetadata :> SemanticMetadata {
		doc /* The `#allocationOfRequirementsToNextLowerLevel` keyword: classifies an item definition or
		 * item usage as a allocationOfRequirementsToNextLowerLevel by applying the definition and
		 * the `allocationOfRequirementsToNextLowerLevels` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = allocationOfRequirementsToNextLowerLevels meta SysML::Usage;
	}

	metadata def <requiredLeadingIndicatorTrends> RequiredLeadingIndicatorTrendsMetadata :> SemanticMetadata {
		doc /* The `#requiredLeadingIndicatorTrends` keyword: classifies an item definition or item
		 * usage as a requiredLeadingIndicatorTrends by applying the definition and the
		 * `requiredLeadingIndicatorTrendss` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = requiredLeadingIndicatorTrendss meta SysML::Usage;
	}

	metadata def <designSolution> DesignSolutionMetadata :> SemanticMetadata {
		doc /* The `#designSolution` keyword: classifies an item definition or item usage as a
		 * designSolution by applying the definition and the `designSolutions` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = designSolutions meta SysML::Usage;
	}

	metadata def <interfaceDefinitionProduct> InterfaceDefinitionProductMetadata :> SemanticMetadata {
		doc /* The `#interfaceDefinitionProduct` keyword: classifies an item definition or item usage as
		 * a interfaceDefinitionProduct by applying the definition and the
		 * `interfaceDefinitionProducts` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = interfaceDefinitionProducts meta SysML::Usage;
	}

	metadata def <implementationPlan> ImplementationPlanMetadata :> SemanticMetadata {
		doc /* The `#implementationPlan` keyword: classifies an item definition or item usage as a
		 * implementationPlan by applying the definition and the `implementationPlans` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = implementationPlans meta SysML::Usage;
	}

	metadata def <integrationPlan> IntegrationPlanMetadata :> SemanticMetadata {
		doc /* The `#integrationPlan` keyword: classifies an item definition or item usage as a
		 * integrationPlan by applying the definition and the `integrationPlans` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = integrationPlans meta SysML::Usage;
	}

	metadata def <verificationAndValidationPlan> VerificationAndValidationPlanMetadata :> SemanticMetadata {
		doc /* The `#verificationAndValidationPlan` keyword: classifies an item definition or item usage
		 * as a verificationAndValidationPlan by applying the definition and the
		 * `verificationAndValidationPlans` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = verificationAndValidationPlans meta SysML::Usage;
	}

	metadata def <verificationAndValidationResults> VerificationAndValidationResultsMetadata :> SemanticMetadata {
		doc /* The `#verificationAndValidationResults` keyword: classifies an item definition or item
		 * usage as a verificationAndValidationResults by applying the definition and the
		 * `verificationAndValidationResultss` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = verificationAndValidationResultss meta SysML::Usage;
	}

	metadata def <transportationCriteriaAndInstructions> TransportationCriteriaAndInstructionsMetadata :> SemanticMetadata {
		doc /* The `#transportationCriteriaAndInstructions` keyword: classifies an item definition or
		 * item usage as a transportationCriteriaAndInstructions by applying the definition and the
		 * `transportationCriteriaAndInstructionss` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = transportationCriteriaAndInstructionss meta SysML::Usage;
	}

	metadata def <certificationForFlightOrUse> CertificationForFlightOrUseMetadata :> SemanticMetadata {
		doc /* The `#certificationForFlightOrUse` keyword: classifies an item definition or item usage
		 * as a certificationForFlightOrUse by applying the definition and the
		 * `certificationForFlightOrUses` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = certificationForFlightOrUses meta SysML::Usage;
	}

	metadata def <decommissioningPlan> DecommissioningPlanMetadata :> SemanticMetadata {
		doc /* The `#decommissioningPlan` keyword: classifies an item definition or item usage as a
		 * decommissioningPlan by applying the definition and the `decommissioningPlans` base usage
		 * at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = decommissioningPlans meta SysML::Usage;
	}

	metadata def <disposalPlan> DisposalPlanMetadata :> SemanticMetadata {
		doc /* The `#disposalPlan` keyword: classifies an item definition or item usage as a
		 * disposalPlan by applying the definition and the `disposalPlans` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = disposalPlans meta SysML::Usage;
	}

	metadata def <semp> SEMPMetadata :> SemanticMetadata {
		doc /* The `#semp` keyword: classifies an part definition or part usage as a semp by applying
		 * the definition and the `semps` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = semps meta SysML::Usage;
	}

	metadata def <technicalAuthority> TechnicalAuthorityMetadata :> SemanticMetadata {
		doc /* The `#technicalAuthority` keyword: classifies an part definition or part usage as a
		 * technicalAuthoritie by applying the definition and the `technicalAuthorities` base usage
		 * at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = technicalAuthorities meta SysML::Usage;
	}

	metadata def <seRequirement> SERequirementMetadata :> SemanticMetadata {
		doc /* The `#seRequirement` keyword: classifies an requirement definition or requirement usage
		 * as a seRequirement by applying the definition and the `seRequirements` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = seRequirements meta SysML::Usage;
	}
}
