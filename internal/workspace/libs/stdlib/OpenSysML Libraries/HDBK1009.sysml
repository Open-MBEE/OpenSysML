library package HDBK1009 {
	doc /* NON-NORMATIVE OpenSysML vocabulary for NASA-HDBK-1009A, 'NASA Systems Modeling Handbook
	 * for Systems Engineering' (Office of the NASA Chief Engineer, approved 2025-03-12), published
	 * at https://standards.nasa.gov/standard/NASA/NASA-HDBK-1009. The Handbook shows how
	 * system modeling in SysML can be integrated with the NASA systems engineering processes of
	 * NPR 7123.1 (section 1.1), for four of them: Stakeholder Expectation
	 * Definition, Technical Requirements Definition, Product Verification and Product Validation
	 * (Table 4.1-1). It is a work of the United States Government and is guidance, not
	 * requirements; the library quotes it where it defines a term, with the section or figure the
	 * text comes from. The library is a community extension: it is not endorsed by NASA and it
	 * replaces neither the Handbook nor NPR 7123.1. The Handbook 'references System Modeling
	 * Language (SysML), Version 1.7' (section 7, notes) and writes each element of its metamodel (Figures 7-1 and
	 * 7-2) with the SysML v1 element or relationship in brackets, adding that 'any other modeling
	 * language may be substituted in the [ ]' and that the metamodel 'will be updated to reflect
	 * any changes needed in the future, for example SysML version 2.0'. This library makes that
	 * substitution for SysML v2. Each element of the metamodel gets a definition specialising its
	 * native SysML v2 type, an abstract base usage subsetting the native base usage, and a
	 * semantic-metadata keyword applying both at once: `#missionObjective requirement obj1;`,
	 * `#vvConfiguration part powerTest;`. Each relationship of the metamodel is stated with the
	 * SysML v2 relationship of the same meaning, so no relationship is a library invention:
	 * [DeriveReqt] is a `#derivation connection` (RequirementDerivation, re-exported), [Refine] a
	 * `#refinement dependency` (ModelingMetadata, re-exported), [Satisfy] a `satisfy`, [Allocate]
	 * an `allocate`, [Trace] a `dependency`, and [Verify] the `verify` of the objective of a
	 * verification case, which is what the Handbook's V&V Case becomes.
	 */

	private import ScalarValues::Integer;
	private import ScalarValues::String;
	private import Metaobjects::SemanticMetadata;
	private import KerML::Root::Element;
	private import Actions::Action;
	private import Actions::actions;
	private import Items::Item;
	private import Items::items;
	private import Parts::Part;
	private import Parts::parts;
	private import Requirements::RequirementCheck;
	private import Requirements::requirementChecks;
	private import UseCases::UseCase;
	private import UseCases::useCases;
	private import $::States::StateAction;
	private import $::States::stateActions;
	private import VerificationCases::VerificationCase;
	private import VerificationCases::verificationCases;
	private import AnalysisCases::AnalysisCase;
	private import AnalysisCases::analysisCases;
	private import StandardViewDefinitions::*;
	private import Views::asElementTable;
	private import DocumentQueries::*;

	public import ModelingMetadata::*;
	public import RequirementDerivation::*;
	public import VerificationCases::VerificationMethodKind;

	/* Section 7, Figure 7-1: the requirements pillar */

	abstract requirement def RequirementsPillarElement :> RequirementCheck {
		doc /* An element of the requirements pillar of the metamodel of Figure 7-1, 'Metamodel Based
		 * on NASA Systems Engineering (SE) Elements and Relationships', each of which the Handbook
		 * models as a [Requirement]. `id` is the identifier the Handbook's tables show in their Id
		 * column ('sys-1', 'ver-1', 'Val-1').
		 */
		attribute id : String[0..1];
	}

	abstract requirement requirementsPillarElements : RequirementsPillarElement[0..*] nonunique :> requirementChecks;

	part def Stakeholder :> Part {
		doc /* Stakeholders [Actor] of Figure 7-1. 'The stakeholders are captured as actor elements and
		 * the stakeholder expectation statements are depicted with a trace relationship to the
		 * stakeholder.' (section 8.1) State the trace as a `dependency` from the expectation
		 * statement, or from a need, goal or objective, to the stakeholder.
		 */
	}

	abstract part stakeholders : Stakeholder[0..*] nonunique :> parts;

	requirement def StakeholderExpectationStatement :> RequirementsPillarElement {
		doc /* Stakeholder Expectation Statement [Requirement] of Figure 7-1, 'Provided By [Trace]' its
		 * stakeholders (section 8.1). 'Stakeholders can influence requirements at any level; hence
		 * the trace can exist at any level as shown with the Stakeholder Expectation Statement.'
		 * (Figure 7-1, notes on the requirements pillar)
		 */
	}

	abstract requirement stakeholderExpectationStatements : StakeholderExpectationStatement[0..*] nonunique :> requirementsPillarElements;

	requirement def Need :> RequirementsPillarElement {
		doc /* Needs [Requirement] of Figure 7-1: 'Provided By [Trace]' the stakeholders and 'Traced To
		 * [Trace]' the stakeholder expectation statements (section 8.1, Figure 8.1-3; section 8.2,
		 * 'Requirements Diagram (req) of Needs, Goals, and Objectives (NGOs)').
		 */
	}

	abstract requirement needs : Need[0..*] nonunique :> requirementsPillarElements;

	requirement def Goal :> RequirementsPillarElement {
		doc /* Goals [Requirement] of Figure 7-1: 'Derives From [DeriveReqt]' the needs and 'Traced To
		 * [Trace]' the stakeholder expectation statements (sections 8.1 and 8.2).
		 */
	}

	abstract requirement goals : Goal[0..*] nonunique :> requirementsPillarElements;

	requirement def Objective :> RequirementsPillarElement {
		doc /* Objectives [Requirement] of Figure 7-1: 'Derives From [DeriveReqt]' the goals and 'Traced
		 * To [Trace]' the stakeholder expectation statements. Measures of effectiveness, system use
		 * cases and mission activities refine the objectives, and validation requirements satisfy
		 * them (Figures 7-1 and 7-2).
		 */
	}

	abstract requirement objectives : Objective[0..*] nonunique :> requirementsPillarElements;

	abstract requirement def TechnicalRequirement :> RequirementsPillarElement {
		doc /* A technical requirement at one level of the design: the System/Subsystem Requirements
		 * and Component Requirement of Figure 7-1, each 'Derives From [DeriveReqt]' the level above
		 * it. `verifyMethod` is the verification attribute of section 8.14: 'The verify method
		 * attribute can be modeled on the requirements or the verification requirement. The modeler
		 * may choose to specify the verification method on the requirement in early life cycle
		 * phases and then to move to the verification requirement at later phases.'
		 */
		attribute verifyMethod : VerificationMethodKind[0..*];
	}

	abstract requirement technicalRequirements : TechnicalRequirement[0..*] nonunique :> requirementsPillarElements;

	requirement def SystemRequirement :> TechnicalRequirement {
		doc /* A system-level requirement, the upper level of the System/Subsystem Requirements
		 * [Requirement] of Figure 7-1: 'Derives From [DeriveReqt]' the objectives (section 8.10,
		 * 'System Requirement Diagram (req)'; section 8.11, 'Requirements Table with
		 * Traceability').
		 */
	}

	abstract requirement systemRequirements : SystemRequirement[0..*] nonunique :> technicalRequirements;

	requirement def SubsystemRequirement :> TechnicalRequirement {
		doc /* A subsystem-level requirement, the lower level of the System/Subsystem Requirements
		 * [Requirement] of Figure 7-1, which 'Decompose [DeriveReqt]': a subsystem requirement
		 * derives from a system requirement or from another subsystem requirement.
		 */
	}

	abstract requirement subsystemRequirements : SubsystemRequirement[0..*] nonunique :> technicalRequirements;

	requirement def ComponentRequirement :> TechnicalRequirement {
		doc /* Component Requirement [Requirement] of Figure 7-1: 'Derives From [DeriveReqt]' the
		 * system/subsystem requirements.
		 */
	}

	abstract requirement componentRequirements : ComponentRequirement[0..*] nonunique :> technicalRequirements;

	requirement def MeasureOfEffectiveness :> RequirementsPillarElement {
		doc /* Measures of Effectiveness (MOE) of Figure 7-1: 'Refines [Refine]' the objectives. State
		 * the refinement as a `#refinement dependency` from the measure to the objective
		 * (section 8.12, 'Requirements Diagram (req) of MOEs, MOPs, and Requirements
		 * Traceability'). The mission or system elements satisfy it.
		 */
	}

	abstract requirement measuresOfEffectiveness : MeasureOfEffectiveness[0..*] nonunique :> requirementsPillarElements;

	requirement def MeasureOfPerformance :> RequirementsPillarElement {
		doc /* Measures of Performance (MOP) of Figure 7-1: 'Derives From [DeriveReqt]' the measures of
		 * effectiveness and 'Refines [Refine]' the system/subsystem requirements (section 8.12).
		 * Technical performance measures satisfy it (section 8.13).
		 */
	}

	abstract requirement measuresOfPerformance : MeasureOfPerformance[0..*] nonunique :> requirementsPillarElements;

	abstract requirement def VVRequirement :> RequirementsPillarElement {
		doc /* A verification or validation requirement, the Verification Requirement/Statement and
		 * Validation Requirement/Statement [Requirement] of Figures 7-1 and 7-2. Its attributes are
		 * the columns the Handbook's verification and validation matrices show beside the
		 * statement (Figures 8.16-1, 8.17-1 and 8.22-1): the method, the success criteria and the
		 * 'Documentation/Results' closure record. A V&V case refines the requirement: make the
		 * requirement the objective of the case (`objective :> ver1 { verify sys1; }`).
		 */
		attribute verifyMethod : VerificationMethodKind[0..*];
		attribute successCriteria : String[0..1];
		attribute closureStatus : String[0..1];
		attribute closureDate : String[0..1];
		attribute closureNotes : String[0..1];
	}

	abstract requirement vvRequirements : VVRequirement[0..*] nonunique :> requirementsPillarElements;

	requirement def VerificationRequirement :> VVRequirement {
		doc /* Verification Requirement/Statement [Requirement] of Figure 7-2: 'Verifies [Verify]' the
		 * system/subsystem requirements (sections 8.14 to 8.17). SysML v2 states a verification in the objective of a verification case, so the
		 * requirements it verifies are the ones the `verify` of its V&V cases name.
		 */
	}

	abstract requirement verificationRequirements : VerificationRequirement[0..*] nonunique :> vvRequirements;

	requirement def ValidationRequirement :> VVRequirement {
		doc /* Validation Requirement/Statement [Requirement] of Figure 7-2: 'Satisfies [Satisfy]' the
		 * objectives (sections 8.21 and 8.22). State it as
		 * `satisfy <objective> by <validation requirement>;`.
		 */
	}

	abstract requirement validationRequirements : ValidationRequirement[0..*] nonunique :> vvRequirements;

	/* Section 7, Figure 7-1: the behavior pillar */

	use case def SystemUseCase :> UseCase {
		doc /* System Use Cases [Use Case] of Figure 7-1: 'Refines [Refine]' the objectives, with the
		 * system users and external elements as its actors (section 8.5, 'System Use Case (uc) Diagram').
		 */
	}

	abstract use case systemUseCases : SystemUseCase[0..*] nonunique :> useCases;

	action def MissionActivity :> Action {
		doc /* Mission Phases and Activities/Capabilities [Activity] of Figure 7-1: 'Refines [Refine]'
		 * the objectives and is 'Allocated To [Allocate]' the mission/system elements (section 8.6,
		 * 'Activity Diagram (act) Supporting Use Case').
		 */
	}

	abstract action missionActivities : MissionActivity[0..*] nonunique :> actions;

	action def SystemFunction :> Action {
		doc /* System/Subsystem Functions [Activity] of Figure 7-1: decomposes the mission
		 * activities, 'Refines [Refine]' and 'Satisfies [Satisfy]' the system/subsystem requirements
		 * and is 'Allocated To [Allocate]' the system/subsystem elements (section 8.9).
		 */
	}

	abstract action systemFunctions : SystemFunction[0..*] nonunique :> actions;

	action def ComponentFunction :> Action {
		doc /* Component Functions [Activity] of Figure 7-1: decomposes the system/subsystem
		 * functions, 'Refines [Refine]' and 'Satisfies [Satisfy]' the component requirements and is
		 * 'Allocated To [Allocate]' the component elements.
		 */
	}

	abstract action componentFunctions : ComponentFunction[0..*] nonunique :> actions;

	state def ComponentStateMachine :> StateAction {
		doc /* Component State Machine [State Machine] of Figure 7-1: the states of a component
		 * element.
		 */
	}

	abstract state componentStateMachines : ComponentStateMachine[0..*] nonunique :> stateActions;

	/* Section 7, Figure 7-1: the structure pillar */

	part def SystemContext :> Part {
		doc /* Mission/System Context [Block] of Figure 7-1: composes the mission/system elements, the
		 * external elements and the system users (section 8.3, 'System Context Block Definition
		 * Diagram (bdd)').
		 */
	}

	abstract part systemContexts : SystemContext[0..*] nonunique :> parts;

	part def ExternalElement :> Part {
		doc /* External Elements [Block] of Figure 7-1: what the system of interest interfaces with
		 * and does not own.
		 */
	}

	abstract part externalElements : ExternalElement[0..*] nonunique :> parts;

	part def SystemUser :> Part {
		doc /* System Users [Actor] of Figure 7-1: the users acting in the system use cases. */
	}

	abstract part systemUsers : SystemUser[0..*] nonunique :> parts;

	part def MissionElement :> Part {
		doc /* Mission/System Elements [Block] of Figure 7-1: 'Satisfies [Satisfy]' the measures of
		 * effectiveness, and has the mission activities allocated to it.
		 */
	}

	abstract part missionElements : MissionElement[0..*] nonunique :> parts;

	part def SystemElement :> Part {
		doc /* System/Subsystem Elements [Block] of Figure 7-1: composed by the mission/system
		 * elements, 'Satisfies [Satisfy]' the system/subsystem requirements (section 8.7,
		 * 'Structural Decomposition Block Definition Diagram (bdd)').
		 */
	}

	abstract part systemElements : SystemElement[0..*] nonunique :> parts;

	part def ComponentElement :> Part {
		doc /* Component Elements [Block] of Figure 7-1: composed by the system/subsystem elements,
		 * 'Satisfies [Satisfy]' the component requirements.
		 */
	}

	abstract part componentElements : ComponentElement[0..*] nonunique :> parts;

	/* Section 7, Figure 7-1: the parametric pillar */

	abstract attribute technicalPerformanceMeasures[0..*] nonunique {
		doc /* Technical Performance Measure [Value Property] of Figure 7-1: a value property the
		 * mission, system/subsystem or component elements have, which 'Satisfies [Satisfy]' a
		 * measure of performance (section 8.13, 'Structural Decomposition bdd with MOP and TPM
		 * Identification'). Mark the attribute usage `#tpm` and state
		 * `satisfy <measure of performance> by <element>.<attribute>;`.
		 */
	}

	analysis def ParametricAnalysis :> AnalysisCase {
		doc /* Component Parametric Analysis [Parametric Diagram] of Figure 7-1: an analysis whose
		 * constraints relate the quantitative and qualitative values of the elements.
		 */
	}

	abstract analysis parametricAnalyses : ParametricAnalysis[0..*] nonunique :> analysisCases;

	/* Section 7, Figure 7-2: the verification and validation metamodel */

	part def VVPersonnel :> Part {
		doc /* V&V Personnel [Block] of Figure 7-2: the people a V&V configuration has and the V&V steps
		 * are 'Allocated To [allocate]'.
		 */
	}

	abstract part vvPersonnelMembers : VVPersonnel[0..*] nonunique :> parts;

	part def VVEquipment :> Part {
		doc /* V&V Equipment of Figure 7-2: the equipment a V&V configuration has and the V&V steps are
		 * 'Allocated To [allocate]'.
		 */
	}

	abstract part vvEquipmentItems : VVEquipment[0..*] nonunique :> parts;

	part def VVFacility :> Part {
		doc /* V&V Facilities of Figure 7-2: the facilities a V&V configuration has and the V&V steps
		 * are 'Allocated To [allocate]'.
		 */
	}

	abstract part vvFacilities : VVFacility[0..*] nonunique :> parts;

	part def VVConfiguration :> Part {
		doc /* V&V Configuration [Block] of Figure 7-2, 'Associated With [Association]' the V&V cases
		 * (section 8.19, 'V&V Configuration Decomposition Block Definition Diagram (bdd)'). It
		 * aggregates the articles under test, which are elements of the structure pillar and so
		 * are referenced, and is composed of the personnel, equipment and facilities.
		 */
		ref part articlesUnderTest : Part[0..*];
		part personnel : VVPersonnel[0..*] :> vvPersonnelMembers;
		part equipment : VVEquipment[0..*] :> vvEquipmentItems;
		part facilities : VVFacility[0..*] :> vvFacilities;
	}

	abstract part vvConfigurations : VVConfiguration[0..*] nonunique :> parts;

	item def VVArtifact :> Item {
		doc /* Artifact for V&V [Artifact] of Figure 7-2: what a V&V step produces, which 'Satisfies
		 * [Satisfy]' the verification or validation requirement. State it as
		 * `satisfy <V&V requirement> by <artifact>;`.
		 */
	}

	abstract item vvArtifacts : VVArtifact[0..*] nonunique :> items;

	action def VVEvent :> Action {
		doc /* V&V Event Activity [Activity] of Figure 7-2: the event the V&V requirements and V&V cases
		 * are 'Assigned to [Allocate]', which includes the V&V case activities. State the
		 * assignment as `allocate <V&V requirement or case> to <event>;`.
		 */
	}

	abstract action vvEvents : VVEvent[0..*] nonunique :> actions;

	action def VVStep :> Action {
		doc /* V&V Step [Activity] of Figure 7-2: a step of a V&V case activity, 'Allocated To
		 * [allocate]' the article under test, personnel, equipment and facilities, which produces
		 * the artifacts for V&V.
		 */
		out item artifacts : VVArtifact[0..*];
	}

	abstract action vvSteps : VVStep[0..*] nonunique :> actions;

	action def VVCaseActivity :> Action {
		doc /* V&V Case Activity [Activity] of Figure 7-2: the behavior of a V&V case, composed of V&V
		 * steps (section 8.20, 'Activity Diagram (act) of V&V Case/ Sequencing of the V&V
		 * Approach').
		 */
	}

	abstract action vvCaseActivities : VVCaseActivity[0..*] nonunique :> actions;

	verification def VVCase :> VerificationCase {
		doc /* V&V Case [Use Case] of Figure 7-2: 'Refines [Refines]' a verification or validation
		 * requirement (section 8.18, 'Use Case (uc) Diagram of V&V Cases'). In SysML v2 it is a
		 * verification case: state the refinement as a `#refinement dependency` from the case to
		 * the requirement, and for a verification name the requirements verified in the objective
		 * (`objective { verify sys1; }`).
		 */
		ref part configurations : VVConfiguration[0..*];
	}

	abstract verification vvCases : VVCase[0..*] nonunique :> verificationCases;

	/* Section 4.1.1, Table 4.1-1: the processes and products the Handbook covers */

	enum def ProductKind {
		doc /* The products of Table 4.1-1, 'NASA SE Processes and Products Covered in Handbook',
		 * grouped there under Stakeholder Expectation Definition, Technical Requirements
		 * Definition, Product Verification and Product Validation.
		 */
		enum stakeholderIdentificationAndExpectationsDefinition;
		enum conceptOfOperations;
		enum measuresOfEffectivenessDefinition;
		enum technicalRequirements;
		enum measuresOfPerformance;
		enum technicalPerformanceMeasuresDefinition;
		enum verificationRequirements;
		enum verificationPlanning;
		enum verificationResultsAndReports;
		enum validationRequirements;
		enum validationPlanning;
		enum validationResultsAndReports;
	}

	/* Sections 8 and 9: the tables the Handbook generates from the model */

	calc def StakeholderRegister :> Query {
		doc /* The stakeholders with their descriptions, after Figure 8.1-2, 'Stakeholder Description
		 * Table'. */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(
				source = WhereType(source = Descendants(source = root, maxDepth = depth), type = "HDBK1009::Stakeholder"),
				type = ("PartUsage", "PartDefinition"), exact = true
			),
			columns = (
				Column(name = "Name", path = "name"),
				Column(name = "Description", path = "documentation")
			)
		)
	}

	calc def ExpectationRegister :> Query {
		doc /* The stakeholder expectation statements, needs, goals and objectives with what each is
		 * traced to and derives from (sections 8.1 and 8.2). 'Traced To' follows every dependency
		 * the row states, so a refinement the row states shows there too. */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(
				source = WhereType(source = Descendants(source = root, maxDepth = depth), type = ("HDBK1009::StakeholderExpectationStatement", "HDBK1009::Need", "HDBK1009::Goal", "HDBK1009::Objective")),
				type = ("RequirementUsage", "RequirementDefinition"), exact = true
			),
			columns = (
				Column(name = "Id", path = "id"),
				Column(name = "Name", path = "name"),
				Column(name = "Text", path = "documentation"),
				RelatedColumn(name = "Traced To", relationshipKind = "dependency", direction = "outgoing", maxDepth = 1),
				RelatedColumn(name = "Derives From", relationshipKind = "derivation", direction = "incoming", maxDepth = 1)
			)
		)
	}

	calc def RequirementsRegister :> Query {
		doc /* The technical requirements with their traceability, after section 8.11, 'Requirements
		 * Table with Traceability'. */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = Except(
				source = WhereType(
					source = WhereType(source = Descendants(source = root, maxDepth = depth), type = "HDBK1009::TechnicalRequirement"),
					type = ("RequirementUsage", "RequirementDefinition"), exact = true
				),
				// A `verify` in the objective of a verification case names a requirement; it is not another one.
				exclude = Descendants(
					source = WhereType(
						source = Descendants(source = root, maxDepth = depth),
						type = ("VerificationCaseUsage", "VerificationCaseDefinition")
					)
				)
			),
			columns = (
				Column(name = "Id", path = "id"),
				Column(name = "Name", path = "name"),
				Column(name = "Text", path = "documentation"),
				RelatedColumn(name = "Derives From", relationshipKind = "derivation", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "Refined By", relationshipKind = "refinement", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "Satisfied By", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1),
				Column(name = "Verify Method", path = "verifyMethod")
			)
		)
	}

	calc def MeasureRegister :> Query {
		doc /* The measures of effectiveness and of performance with what each refines and derives
		 * from and what satisfies it (sections 8.12 and 8.13). */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(
				source = WhereType(source = Descendants(source = root, maxDepth = depth), type = ("HDBK1009::MeasureOfEffectiveness", "HDBK1009::MeasureOfPerformance")),
				type = ("RequirementUsage", "RequirementDefinition"), exact = true
			),
			columns = (
				Column(name = "Id", path = "id"),
				Column(name = "Name", path = "name"),
				Column(name = "Text", path = "documentation"),
				RelatedColumn(name = "Refines", relationshipKind = "refinement", direction = "outgoing", maxDepth = 1),
				RelatedColumn(name = "Derives From", relationshipKind = "derivation", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "Satisfied By", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1)
			)
		)
	}

	calc def RequirementsVerificationRegister :> Query {
		doc /* The requirements with the V&V cases verifying them, after Figure 8.16-1 (section 8.16,
		 * 'Table for Requirements Verification Matrix'). */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = Except(
				source = WhereType(
					source = WhereType(source = Descendants(source = root, maxDepth = depth), type = "HDBK1009::TechnicalRequirement"),
					type = ("RequirementUsage", "RequirementDefinition"), exact = true
				),
				// A `verify` in the objective of a verification case names a requirement; it is not another one.
				exclude = Descendants(
					source = WhereType(
						source = Descendants(source = root, maxDepth = depth),
						type = ("VerificationCaseUsage", "VerificationCaseDefinition")
					)
				)
			),
			columns = (
				Column(name = "Id", path = "id"),
				Column(name = "Name", path = "name"),
				Column(name = "Text", path = "documentation"),
				RelatedColumn(name = "Verified By", relationshipKind = "verification", direction = "incoming", maxDepth = 1),
				Column(name = "Verify Method", path = "verifyMethod")
			)
		)
	}

	calc def VerificationRequirementsRegister :> Query {
		doc /* The verification requirements, after Figure 8.17-1 (section 8.17, 'Table for
		 * Verification Requirements Matrix/ Verification Compliance Spreadsheet (VCS)'). The items
		 * verified are the ones the V&V cases under 'Refined By' verify, which
		 * RequirementsVerificationRegister lists by requirement. */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = Except(
				source = WhereType(
					source = WhereType(source = Descendants(source = root, maxDepth = depth), type = "HDBK1009::VerificationRequirement"),
					type = ("RequirementUsage", "RequirementDefinition"), exact = true
				),
				// An objective subsetting the requirement (`objective :> ver1`) is the case's, not another requirement.
				exclude = Descendants(
					source = WhereType(
						source = Descendants(source = root, maxDepth = depth),
						type = ("VerificationCaseUsage", "VerificationCaseDefinition")
					)
				)
			),
			columns = (
				Column(name = "Id", path = "id"),
				Column(name = "Name", path = "name"),
				Column(name = "Verify Method", path = "verifyMethod"),
				Column(name = "Text", path = "documentation"),
				Column(name = "Success Criteria", path = "successCriteria"),
				RelatedColumn(name = "Refined By", relationshipKind = "refinement", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "Artifact", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "Verification Event", relationshipKind = "allocation", direction = "outgoing", maxDepth = 1),
				Column(name = "Closure Status", path = "closureStatus"),
				Column(name = "Closure Date", path = "closureDate"),
				Column(name = "Closure Notes", path = "closureNotes")
			)
		)
	}

	calc def ValidationRequirementsRegister :> Query {
		doc /* The validation requirements, after Figure 8.22-1 (section 8.22, 'Table for Validation
		 * Requirements Matrix/ Validation Compliance Spreadsheet (VCS)'). */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = Except(
				source = WhereType(
					source = WhereType(source = Descendants(source = root, maxDepth = depth), type = "HDBK1009::ValidationRequirement"),
					type = ("RequirementUsage", "RequirementDefinition"), exact = true
				),
				// An objective subsetting the requirement (`objective :> ver1`) is the case's, not another requirement.
				exclude = Descendants(
					source = WhereType(
						source = Descendants(source = root, maxDepth = depth),
						type = ("VerificationCaseUsage", "VerificationCaseDefinition")
					)
				)
			),
			columns = (
				RelatedColumn(name = "Validation Product", relationshipKind = "satisfaction", direction = "outgoing", maxDepth = 1),
				Column(name = "Id", path = "id"),
				Column(name = "Name", path = "name"),
				Column(name = "Text", path = "documentation"),
				Column(name = "Success Criteria", path = "successCriteria"),
				RelatedColumn(name = "Refined By", relationshipKind = "refinement", direction = "incoming", maxDepth = 1),
				Column(name = "Validation Method", path = "verifyMethod"),
				RelatedColumn(name = "Validation Artifact", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "Validation Event", relationshipKind = "allocation", direction = "outgoing", maxDepth = 1),
				Column(name = "Closure Status", path = "closureStatus"),
				Column(name = "Closure Date", path = "closureDate"),
				Column(name = "Closure Notes", path = "closureNotes")
			)
		)
	}

	abstract part def ProductDocument :> Document {
		doc /* A document generated from the model for one of the products of Table 4.1-1. */
		:>> title default = "NASA-HDBK-1009A product";
		attribute product : ProductKind[0..1];
	}

	abstract part def StakeholderExpectations :> ProductDocument {
		doc /* The document for the stakeholder identification and expectations definition, whose tables are StakeholderRegister and
		 * ExpectationRegister (section 9.1).
		 */
		:>> title default = "Stakeholder Identification and Expectations";
		:>> product default = ProductKind::stakeholderIdentificationAndExpectationsDefinition;
	}

	abstract part def TechnicalRequirementsDocument :> ProductDocument {
		doc /* The document for the technical requirements, whose table is RequirementsRegister (section 9.4).
		 */
		:>> title default = "Technical Requirements";
		:>> product default = ProductKind::technicalRequirements;
	}

	abstract part def MeasuresDocument :> ProductDocument {
		doc /* The document for the measures of effectiveness and of performance, whose table is MeasureRegister (sections
		 * 9.3 and 9.5).
		 */
		:>> title default = "Measures of Effectiveness and Performance";
		:>> product default = ProductKind::measuresOfPerformance;
	}

	abstract part def RequirementsVerificationMatrix :> ProductDocument {
		doc /* The document for the requirements verification matrix, whose table is RequirementsVerificationRegister
		 * (section 8.16).
		 */
		:>> title default = "Requirements Verification Matrix";
		:>> product default = ProductKind::verificationPlanning;
	}

	abstract part def VerificationRequirementsMatrix :> ProductDocument {
		doc /* The document for the verification requirements matrix or verification compliance spreadsheet, whose table
		 * is VerificationRequirementsRegister (section 8.17).
		 */
		:>> title default = "Verification Requirements Matrix";
		:>> product default = ProductKind::verificationRequirements;
	}

	abstract part def ValidationRequirementsMatrix :> ProductDocument {
		doc /* The document for the validation requirements matrix or validation compliance spreadsheet, whose table is
		 * ValidationRequirementsRegister (section 8.22).
		 */
		:>> title default = "Validation Requirements Matrix";
		:>> product default = ProductKind::validationRequirements;
	}

	view def StakeholderView :> GridView {
		doc /* Table of the stakeholders. */
		filter @StakeholderMetadata;
		render asElementTable;
	}

	view def ObjectiveView :> GridView {
		doc /* Table of the objectives. */
		filter @ObjectiveMetadata;
		render asElementTable;
	}

	view def SystemRequirementView :> GridView {
		doc /* Table of the system requirements. */
		filter @SystemRequirementMetadata;
		render asElementTable;
	}

	view def VerificationRequirementView :> GridView {
		doc /* Table of the verification requirements. */
		filter @VerificationRequirementMetadata;
		render asElementTable;
	}

	view def ValidationRequirementView :> GridView {
		doc /* Table of the validation requirements. */
		filter @ValidationRequirementMetadata;
		render asElementTable;
	}

	view def VVCaseView :> GridView {
		doc /* Table of the V&V cases. */
		filter @VVCaseMetadata;
		render asElementTable;
	}

	/* Keywords */

	metadata def <stakeholderActor> StakeholderMetadata :> SemanticMetadata {
		doc /* The `#stakeholderActor` keyword: classifies a definition or usage as a Stakeholder by applying the
		 * definition and the `stakeholders` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = stakeholders meta SysML::Usage;
	}

	metadata def <stakeholderExpectationStatement> StakeholderExpectationStatementMetadata :> SemanticMetadata {
		doc /* The `#stakeholderExpectationStatement` keyword: classifies a definition or usage as a StakeholderExpectationStatement by applying the
		 * definition and the `stakeholderExpectationStatements` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = stakeholderExpectationStatements meta SysML::Usage;
	}

	metadata def <need> NeedMetadata :> SemanticMetadata {
		doc /* The `#need` keyword: classifies a definition or usage as a Need by applying the
		 * definition and the `needs` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = needs meta SysML::Usage;
	}

	metadata def <goal> GoalMetadata :> SemanticMetadata {
		doc /* The `#goal` keyword: classifies a definition or usage as a Goal by applying the
		 * definition and the `goals` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = goals meta SysML::Usage;
	}

	metadata def <missionObjective> ObjectiveMetadata :> SemanticMetadata {
		doc /* The `#missionObjective` keyword: classifies a definition or usage as an Objective by applying the
		 * definition and the `objectives` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = objectives meta SysML::Usage;
	}

	metadata def <systemRequirement> SystemRequirementMetadata :> SemanticMetadata {
		doc /* The `#systemRequirement` keyword: classifies a definition or usage as a SystemRequirement by applying the
		 * definition and the `systemRequirements` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = systemRequirements meta SysML::Usage;
	}

	metadata def <subsystemRequirement> SubsystemRequirementMetadata :> SemanticMetadata {
		doc /* The `#subsystemRequirement` keyword: classifies a definition or usage as a SubsystemRequirement by applying the
		 * definition and the `subsystemRequirements` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = subsystemRequirements meta SysML::Usage;
	}

	metadata def <componentRequirement> ComponentRequirementMetadata :> SemanticMetadata {
		doc /* The `#componentRequirement` keyword: classifies a definition or usage as a ComponentRequirement by applying the
		 * definition and the `componentRequirements` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = componentRequirements meta SysML::Usage;
	}

	metadata def <measureOfEffectiveness> MeasureOfEffectivenessMetadata :> SemanticMetadata {
		doc /* The `#measureOfEffectiveness` keyword: classifies a definition or usage as a MeasureOfEffectiveness by applying the
		 * definition and the `measuresOfEffectiveness` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = measuresOfEffectiveness meta SysML::Usage;
	}

	metadata def <measureOfPerformance> MeasureOfPerformanceMetadata :> SemanticMetadata {
		doc /* The `#measureOfPerformance` keyword: classifies a definition or usage as a MeasureOfPerformance by applying the
		 * definition and the `measuresOfPerformance` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = measuresOfPerformance meta SysML::Usage;
	}

	metadata def <verificationRequirement> VerificationRequirementMetadata :> SemanticMetadata {
		doc /* The `#verificationRequirement` keyword: classifies a definition or usage as a VerificationRequirement by applying the
		 * definition and the `verificationRequirements` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = verificationRequirements meta SysML::Usage;
	}

	metadata def <validationRequirement> ValidationRequirementMetadata :> SemanticMetadata {
		doc /* The `#validationRequirement` keyword: classifies a definition or usage as a ValidationRequirement by applying the
		 * definition and the `validationRequirements` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = validationRequirements meta SysML::Usage;
	}

	metadata def <systemUseCase> SystemUseCaseMetadata :> SemanticMetadata {
		doc /* The `#systemUseCase` keyword: classifies a definition or usage as a SystemUseCase by applying the
		 * definition and the `systemUseCases` base usage at once.
		 */
		:> annotatedElement : SysML::UseCaseDefinition;
		:> annotatedElement : SysML::UseCaseUsage;
		:>> baseType = systemUseCases meta SysML::Usage;
	}

	metadata def <missionActivity> MissionActivityMetadata :> SemanticMetadata {
		doc /* The `#missionActivity` keyword: classifies a definition or usage as a MissionActivity by applying the
		 * definition and the `missionActivities` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = missionActivities meta SysML::Usage;
	}

	metadata def <systemFunction> SystemFunctionMetadata :> SemanticMetadata {
		doc /* The `#systemFunction` keyword: classifies a definition or usage as a SystemFunction by applying the
		 * definition and the `systemFunctions` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = systemFunctions meta SysML::Usage;
	}

	metadata def <componentFunction> ComponentFunctionMetadata :> SemanticMetadata {
		doc /* The `#componentFunction` keyword: classifies a definition or usage as a ComponentFunction by applying the
		 * definition and the `componentFunctions` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = componentFunctions meta SysML::Usage;
	}

	metadata def <componentStateMachine> ComponentStateMachineMetadata :> SemanticMetadata {
		doc /* The `#componentStateMachine` keyword: classifies a definition or usage as a ComponentStateMachine by applying the
		 * definition and the `componentStateMachines` base usage at once.
		 */
		:> annotatedElement : SysML::StateDefinition;
		:> annotatedElement : SysML::StateUsage;
		:>> baseType = componentStateMachines meta SysML::Usage;
	}

	metadata def <systemContext> SystemContextMetadata :> SemanticMetadata {
		doc /* The `#systemContext` keyword: classifies a definition or usage as a SystemContext by applying the
		 * definition and the `systemContexts` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = systemContexts meta SysML::Usage;
	}

	metadata def <externalElement> ExternalElementMetadata :> SemanticMetadata {
		doc /* The `#externalElement` keyword: classifies a definition or usage as an ExternalElement by applying the
		 * definition and the `externalElements` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = externalElements meta SysML::Usage;
	}

	metadata def <systemUser> SystemUserMetadata :> SemanticMetadata {
		doc /* The `#systemUser` keyword: classifies a definition or usage as a SystemUser by applying the
		 * definition and the `systemUsers` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = systemUsers meta SysML::Usage;
	}

	metadata def <missionElement> MissionElementMetadata :> SemanticMetadata {
		doc /* The `#missionElement` keyword: classifies a definition or usage as a MissionElement by applying the
		 * definition and the `missionElements` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = missionElements meta SysML::Usage;
	}

	metadata def <systemElement> SystemElementMetadata :> SemanticMetadata {
		doc /* The `#systemElement` keyword: classifies a definition or usage as a SystemElement by applying the
		 * definition and the `systemElements` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = systemElements meta SysML::Usage;
	}

	metadata def <componentElement> ComponentElementMetadata :> SemanticMetadata {
		doc /* The `#componentElement` keyword: classifies a definition or usage as a ComponentElement by applying the
		 * definition and the `componentElements` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = componentElements meta SysML::Usage;
	}

	metadata def <parametricAnalysis> ParametricAnalysisMetadata :> SemanticMetadata {
		doc /* The `#parametricAnalysis` keyword: classifies a definition or usage as a ParametricAnalysis by applying the
		 * definition and the `parametricAnalyses` base usage at once.
		 */
		:> annotatedElement : SysML::AnalysisCaseDefinition;
		:> annotatedElement : SysML::AnalysisCaseUsage;
		:>> baseType = parametricAnalyses meta SysML::Usage;
	}

	metadata def <vvPersonnel> VVPersonnelMetadata :> SemanticMetadata {
		doc /* The `#vvPersonnel` keyword: classifies a definition or usage as a VVPersonnel by applying the
		 * definition and the `vvPersonnelMembers` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = vvPersonnelMembers meta SysML::Usage;
	}

	metadata def <vvEquipment> VVEquipmentMetadata :> SemanticMetadata {
		doc /* The `#vvEquipment` keyword: classifies a definition or usage as a VVEquipment by applying the
		 * definition and the `vvEquipmentItems` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = vvEquipmentItems meta SysML::Usage;
	}

	metadata def <vvFacility> VVFacilityMetadata :> SemanticMetadata {
		doc /* The `#vvFacility` keyword: classifies a definition or usage as a VVFacility by applying the
		 * definition and the `vvFacilities` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = vvFacilities meta SysML::Usage;
	}

	metadata def <vvConfiguration> VVConfigurationMetadata :> SemanticMetadata {
		doc /* The `#vvConfiguration` keyword: classifies a definition or usage as a VVConfiguration by applying the
		 * definition and the `vvConfigurations` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = vvConfigurations meta SysML::Usage;
	}

	metadata def <vvArtifact> VVArtifactMetadata :> SemanticMetadata {
		doc /* The `#vvArtifact` keyword: classifies a definition or usage as a VVArtifact by applying the
		 * definition and the `vvArtifacts` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = vvArtifacts meta SysML::Usage;
	}

	metadata def <vvEvent> VVEventMetadata :> SemanticMetadata {
		doc /* The `#vvEvent` keyword: classifies a definition or usage as a VVEvent by applying the
		 * definition and the `vvEvents` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = vvEvents meta SysML::Usage;
	}

	metadata def <vvStep> VVStepMetadata :> SemanticMetadata {
		doc /* The `#vvStep` keyword: classifies a definition or usage as a VVStep by applying the
		 * definition and the `vvSteps` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = vvSteps meta SysML::Usage;
	}

	metadata def <vvCaseActivity> VVCaseActivityMetadata :> SemanticMetadata {
		doc /* The `#vvCaseActivity` keyword: classifies a definition or usage as a VVCaseActivity by applying the
		 * definition and the `vvCaseActivities` base usage at once.
		 */
		:> annotatedElement : SysML::ActionDefinition;
		:> annotatedElement : SysML::ActionUsage;
		:>> baseType = vvCaseActivities meta SysML::Usage;
	}

	metadata def <vvCase> VVCaseMetadata :> SemanticMetadata {
		doc /* The `#vvCase` keyword: classifies a definition or usage as a VVCase by applying the
		 * definition and the `vvCases` base usage at once.
		 */
		:> annotatedElement : SysML::VerificationCaseDefinition;
		:> annotatedElement : SysML::VerificationCaseUsage;
		:>> baseType = vvCases meta SysML::Usage;
	}

	metadata def <tpm> TechnicalPerformanceMeasureMetadata :> SemanticMetadata {
		doc /* The `#tpm` keyword: marks an attribute usage as a technical performance measure by
		 * applying the `technicalPerformanceMeasures` base usage.
		 */
		:> annotatedElement : SysML::AttributeUsage;
		:>> baseType = technicalPerformanceMeasures meta SysML::Usage;
	}
}
