library package MOSA {
	doc
	/*
	 * NON-NORMATIVE OpenSysML vocabulary for the Modular Open Systems Approach (MOSA), the
	 * integrated business and technical strategy 10 U.S.C. § 4401 requires of United States
	 * Department of Defense major defense acquisition programs. Its concepts and their names
	 * are the statute's: the major system platform, the major system components mounted on
	 * it, the modular systems that can be separated and recombined, and the modular system
	 * interfaces between them, each verified against a widely supported, consensus-based
	 * standard or documented in machine-readable form, with the rights in technical data that
	 * let a component be competed, upgraded or replaced over the life cycle.
	 * Each MOSA artefact gets a definition specialising its native SysML type, a base usage, and
	 * a semantic-metadata keyword applying both at once: `#majorSystemComponent part radar : Radar;`.
	 * Standards conformance, data rights, interface control and conformance assessment are
	 * metadata over the standard model. The library's products are view definitions on the
	 * standard view definitions and document queries for an interface control document.
	 * Measures, requirement derivation and status/rationale metadata reuse the standard domain
	 * libraries, re-exported here.
	 */

	private import ScalarValues::Integer;
	private import ScalarValues::String;
	private import Metaobjects::SemanticMetadata;
	private import KerML::Root::Element;
	private import Parts::Part;
	private import Parts::parts;
	private import Items::Item;
	private import Items::items;
	private import Interfaces::Interface;
	private import Interfaces::interfaces;
	private import Interfaces::BinaryInterface;
	private import Interfaces::binaryInterfaces;
	private import Requirements::RequirementCheck;
	private import Requirements::requirementChecks;
	private import StandardViewDefinitions::*;
	private import Views::asTreeDiagram;
	private import Views::asInterconnectionDiagram;
	private import Views::asElementTable;
	private import DocumentQueries::*;

	public import ParametersOfInterestMetadata::*;
	public import RequirementDerivation::*;
	public import ModelingMetadata::*;

	/* Modular design: the statutory structure of a major system */

	part def MajorSystemPlatform :> Part {
		doc /* A MajorSystemPlatform is the highest-level structure of a major weapon system that is not
		 * itself mounted on a higher-level structure, and on which major system components are
		 * mounted or installed (10 U.S.C. § 4401(b)). */
	}

	abstract part majorSystemPlatforms : MajorSystemPlatform[0..*] nonunique :> parts;

	part def MajorSystemComponent :> Part {
		doc /* A MajorSystemComponent is a high-level subsystem or assembly — hardware, software or an
		 * integrated assembly of components — that is mounted on or installed in a major system
		 * platform, or is itself a major system, and that is designed to be severable and replaced
		 * incrementally over the life cycle. */
	}

	abstract part majorSystemComponents : MajorSystemComponent[0..*] nonunique :> parts;

	part def ModularSystem :> Part {
		doc /* A ModularSystem is a system or component that executes without requiring the coincident
		 * execution of other systems or components, communicates across component boundaries through
		 * interfaces, and functions as a module that can be separated, recombined and connected with
		 * other systems or components to achieve various effects, missions or capabilities.
		 * It is what the MOSA Implementation Guidebook calls a MOSA module. */
	}

	abstract part modularSystems : ModularSystem[0..*] nonunique :> parts;

	/* Modular system interfaces: the shared boundaries a modular design designates */

	enum def InterfaceCharacteristicKind {
		doc /* The physical, logical and functional characteristics the statute names as defining a
		 * modular system interface. An interface may have several. */
		electrical;
		mechanical;
		fluidic;
		optical;
		radioFrequency;
		data;
		networking;
		software;
	}

	interface def ModularSystemInterface :> Interface {
		doc /* A ModularSystemInterface is a shared boundary between major systems, major system
		 * components or modular systems, defined by physical, logical and functional characteristics
		 * such as electrical, mechanical, fluidic, optical, radio-frequency, data, networking or
		 * software elements. MOSA verifies each against a consensus-based standard or documents it
		 * in machine-readable form: the interface's ports, items and attributes are that
		 * documentation, and a StandardConformance names the standards.
		 * The `#modularSystemInterface` keyword declares the binary case, which is the usual one;
		 * an interface with more than two ends specializes this definition directly. */
		attribute characteristics : InterfaceCharacteristicKind[0..*];
	}

	abstract interface modularSystemInterfaces : ModularSystemInterface[0..*] nonunique :> interfaces;

	interface def BinaryModularSystemInterface :> ModularSystemInterface, BinaryInterface {
		doc /* A BinaryModularSystemInterface is a modular system interface between two ports. */
	}

	abstract interface binaryModularSystemInterfaces : BinaryModularSystemInterface[0..*] nonunique :> modularSystemInterfaces, binaryInterfaces;

	/* Open standards and conformance to them */

	item def Standard :> Item {
		doc /* A Standard is a published specification an interface, module or data item can be verified
		 * against. Where the standard's text is held in a standards registry or interface repository,
		 * registry holds a machine-readable reference to that entry. */
		attribute organization : String[0..1];
		attribute identifier : String[0..1];
		attribute version : String[0..1];
		attribute registry : String[0..1];
	}

	abstract item standards : Standard[0..*] nonunique :> items;

	item def ConsensusStandard :> Standard {
		doc /* A ConsensusStandard is a widely supported, consensus-based standard — an open standard in
		 * the guidebook's terms — developed and maintained by a recognised standards body with the
		 * participation of the market its interfaces are competed in. Verification against one is
		 * what the statute asks of a modular system interface. */
	}

	abstract item consensusStandards : ConsensusStandard[0..*] nonunique :> standards;

	ref conformingElements[*] {
		doc /* conformingElements are the interfaces, modules or items that conform in a StandardConformance. */
	}

	ref conformedStandards : Standard[*] {
		doc /* conformedStandards are the standards conformed to in a StandardConformance. */
	}

	abstract connection def StandardConformance {
		doc /* A StandardConformance states that its conforming elements are verified against its standards.
		 * Write one as a connection with a `#conformant` end and a `#conformsTo` end:
		 * `#conformance connection { end #conformant ::> dataLink; end #conformsTo ::> ethernet; }`. */
		ref conformingElement[1..*] :>> conformingElements :> participant {
			doc /* The elements that conform. */
		}
		ref conformedStandard[1..*] :>> conformedStandards :> participant {
			doc /* The standards they conform to. */
		}
	}

	abstract connection standardConformances : StandardConformance[0..*] nonunique {
		doc /* standardConformances is the base feature for StandardConformance connection usages. */
	}

	/* Openness: data rights and proprietary elements */

	enum def DataRightsKind {
		doc /* The license categories for technical data and computer software the Defense Federal
		 * Acquisition Regulation Supplement provides for, which decide whether a component's technical
		 * data can be used to compete, upgrade or replace it. */
		unlimited;
		governmentPurpose;
		limited;
		restricted;
		smallBusinessInnovationResearch;
		speciallyNegotiated;
	}

	metadata def DataRights {
		doc /* DataRights records the rights the acquiring organisation holds in the technical data and
		 * software of the annotated element, who asserted any restriction, and the basis of the assertion. */
		attribute kind : DataRightsKind;
		attribute asserter : String[0..1];
		attribute basis : String[0..1];
	}

	metadata def <proprietary> Proprietary {
		doc /* Proprietary marks an element — an interface, module, standard or data item — that a vendor
		 * controls rather than a consensus standard, with the rationale MOSA asks for keeping it so. */
		attribute owner : String[0..1];
		attribute rationale : String[0..1];
	}

	metadata def InterfaceControl {
		doc /* InterfaceControl records the authority that controls changes to the annotated interface
		 * and the interface control document, if any, that publishes it. */
		attribute authority : String;
		attribute document : String[0..1];
	}

	/* Requirements the strategy states */

	requirement def MOSAObjective :> RequirementCheck {
		doc /* A MOSAObjective states what the program expects its modular open systems approach to
		 * achieve — competition, incremental upgrade, interoperability, reuse or reduced life-cycle cost —
		 * with the measures that show it did. Modularity and interface requirements derive from it. */
	}

	abstract requirement mosaObjectives : MOSAObjective[0..*] nonunique :> requirementChecks;

	requirement def ModularityRequirement :> RequirementCheck {
		doc /* A ModularityRequirement states how the system partitions into modules: which components are
		 * severable, what may be replaced independently of what, and the coupling permitted between them. */
	}

	abstract requirement modularityRequirements : ModularityRequirement[0..*] nonunique :> requirementChecks;

	requirement def InterfaceRequirement :> RequirementCheck {
		doc /* An InterfaceRequirement states what a modular system interface must provide, the standard
		 * it must conform to, or the documentation and rights that must accompany it. */
	}

	abstract requirement interfaceRequirements : InterfaceRequirement[0..*] nonunique :> requirementChecks;

	/* Conformance assessment */

	enum def MOSAConformanceCriterion {
		doc /* The criteria a MOSA conformance assessment judges a program against, after the assessment
		 * criteria of the MOSA Implementation Guidebook. */
		objectives;
		keyInterfaces;
		standardsCompliance;
		modules;
		modularity;
		openness;
		lifeCycleCharacteristics;
		conformance;
	}

	metadata def MOSAConformance {
		doc /* MOSAConformance records the assessed status of the annotated element or package against one
		 * conformance criterion, with the evidence the assessment rests on. */
		attribute criterion : MOSAConformanceCriterion;
		attribute status : StatusKind;
		attribute evidence : String[0..1];
	}

	/* Package template: MOSAPackage records which pillar of the approach a package serves. */

	enum def MOSAPillar {
		doc /* The five pillars of the MOSA Implementation Guidebook. */
		modularDesign;
		keyInterfaces;
		openStandards;
		conformance;
		enablingEnvironment;
	}

	metadata def MOSAPackage {
		doc /* Classifies a package by the MOSA pillar whose artefacts it holds. */
		:> annotatedElement : SysML::Package;
		attribute pillar : MOSAPillar;
		attribute description : String[0..1];
	}

	/* Viewpoints and views: the approach's products, as view conditions over its keywords */

	concern def ModularityConcern {
		doc /* How the system partitions into a platform, major system components and modular systems that
		 * can be separated, replaced and recombined. */
	}

	concern def InterfaceOpennessConcern {
		doc /* Which boundaries between modules are designated modular system interfaces, and whether each is
		 * verified against a consensus standard or documented and controlled in the open. */
	}

	concern def TechnicalDataRightsConcern {
		doc /* Whether the rights held in each component's technical data let it be competed, upgraded or
		 * replaced, and which elements remain proprietary and why. */
	}

	concern def ConformanceConcern {
		doc /* Whether the program's objectives, modules, interfaces and standards meet the conformance
		 * criteria of the approach. */
	}

	viewpoint def ModularDesignViewpoint {
		doc /* The modular structure of the system, as the architect and the program office see it. */
		frame concern : ModularityConcern;
	}

	viewpoint def ModularInterfaceViewpoint {
		doc /* The modular system interfaces and the standards they conform to, as an interface control
		 * working group sees them. */
		frame concern : InterfaceOpennessConcern;
	}

	viewpoint def TechnicalDataViewpoint {
		doc /* Data rights and proprietary elements, as the contracting officer and the competition
		 * planner see them. */
		frame concern : TechnicalDataRightsConcern;
	}

	viewpoint def ConformanceViewpoint {
		doc /* The MOSA requirements and the assessed conformance against them. */
		frame concern : ConformanceConcern;
	}

	view def ModularDecompositionView :> BrowserView {
		doc /* Tree of the major system platform, its major system components and their modular systems. */
		satisfy ModularDesignViewpoint;
		filter @MajorSystemPlatformMetadata or @MajorSystemComponentMetadata or @ModularSystemMetadata;
		render asTreeDiagram;
	}

	view def ModularInterfaceView :> InterconnectionView {
		doc /* Interconnection diagram of the major system components and modular systems with the modular
		 * system interfaces between them. */
		satisfy ModularInterfaceViewpoint;
		filter @MajorSystemPlatformMetadata or @MajorSystemComponentMetadata or @ModularSystemMetadata or @ModularSystemInterfaceMetadata;
		render asInterconnectionDiagram;
	}

	view def InterfaceRegisterView :> GridView {
		doc /* Table of the modular system interfaces. */
		satisfy ModularInterfaceViewpoint;
		filter @ModularSystemInterfaceMetadata;
		render asElementTable;
	}

	view def StandardsView :> GridView {
		doc /* Table of the standards the model names, consensus-based or not. */
		satisfy ModularInterfaceViewpoint;
		filter @StandardMetadata or @ConsensusStandardMetadata;
		render asElementTable;
	}

	view def ProprietaryElementsView :> GridView {
		doc /* Table of the elements marked proprietary. */
		satisfy TechnicalDataViewpoint;
		filter @Proprietary;
		render asElementTable;
	}

	view def DataRightsView :> GridView {
		doc /* Table of the elements whose data rights are recorded. */
		satisfy TechnicalDataViewpoint;
		filter @DataRights;
		render asElementTable;
	}

	view def MOSARequirementsView :> GridView {
		doc /* Table of the MOSA objectives and the modularity and interface requirements. */
		satisfy ConformanceViewpoint;
		filter @MOSAObjectiveMetadata or @ModularityRequirementMetadata or @InterfaceRequirementMetadata;
		render asElementTable;
	}

	view def ConformanceView :> GridView {
		doc /* Table of the elements carrying a conformance assessment. */
		satisfy ConformanceViewpoint;
		filter @MOSAConformance;
		render asElementTable;
	}

	/* Document queries: the registers an interface control document tabulates. Each walks depth
	 * ownership levels below root and omits what lies deeper; a document binds root and, if need be, depth. */

	calc def ModularSystemInterfaces :> Query {
		doc /* The modular system interfaces declared below root: every interface definition or usage
		 * that conforms to ModularSystemInterface, whether it carries the keyword itself or is typed by
		 * a definition that does. */
		in root : Element;
		in depth : Integer = 16;
		WhereType(
			source = Descendants(source = root, maxDepth = depth),
			type = "MOSA::ModularSystemInterface"
		)
	}

	calc def InterfaceRegister :> Query {
		doc /* One row per modular system interface below root: name, type, characteristics and documentation. */
		in root : Element;
		in depth : Integer = 16;
		Project(
			source = ModularSystemInterfaces(root = root, depth = depth),
			properties = ("name", "type", "characteristics", "documentation")
		)
	}

	calc def Standards :> Query {
		doc /* The standards declared below root: every item that conforms to Standard. */
		in root : Element;
		in depth : Integer = 16;
		WhereType(
			source = Descendants(source = root, maxDepth = depth),
			type = "MOSA::Standard"
		)
	}

	calc def StandardsRegister :> Query {
		doc /* One row per standard below root: name, organisation, identifier, version and registry entry. */
		in root : Element;
		in depth : Integer = 16;
		Project(
			source = Standards(root = root, depth = depth),
			properties = ("name", "organization", "identifier", "version", "registry")
		)
	}

	calc def ProprietaryElements :> Query {
		doc /* The elements below root marked proprietary. */
		in root : Element;
		in depth : Integer = 16;
		WhereMetadata(
			source = Descendants(source = root, maxDepth = depth),
			'metadata' = "MOSA::Proprietary"
		)
	}

	calc def ProprietaryRegister :> Query {
		doc /* One row per proprietary element below root: name, type and documentation. */
		in root : Element;
		in depth : Integer = 16;
		Project(
			source = ProprietaryElements(root = root, depth = depth),
			properties = ("name", "type", "documentation")
		)
	}

	calc def DataRightsRegister :> Query {
		doc /* One row per element below root whose data rights are recorded: name, type and documentation. */
		in root : Element;
		in depth : Integer = 16;
		Project(
			source = WhereMetadata(
				source = Descendants(source = root, maxDepth = depth),
				'metadata' = "MOSA::DataRights"
			),
			properties = ("name", "type", "documentation")
		)
	}

	/* Interface control document: a document definition to specialize, with the registers above as its tables */

	abstract part def InterfaceControlDocument :> Document {
		doc /* An InterfaceControlDocument publishes the modular system interfaces of a system, the
		 * standards they conform to and the rights held in them, as MOSA asks of a program's interface
		 * documentation. Specialize it and add sections whose tables run InterfaceRegister,
		 * StandardsRegister, DataRightsRegister and ProprietaryRegister over the system's root:
		 * `part def VehicleICD :> InterfaceControlDocument { part interfaces : Section { part register : Table {
		 * calc rows : InterfaceRegister { in :>> root = vehicle; } } } }`. */
		:>> title default = "Interface Control Document";
	}

	/* Semantic metadata: the user-defined keywords of the approach */

	metadata def <majorSystemPlatform> MajorSystemPlatformMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = majorSystemPlatforms meta SysML::Usage;
	}

	metadata def <majorSystemComponent> MajorSystemComponentMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = majorSystemComponents meta SysML::Usage;
	}

	metadata def <modularSystem> ModularSystemMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = modularSystems meta SysML::Usage;
	}

	metadata def <modularSystemInterface> ModularSystemInterfaceMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::InterfaceDefinition;
		:> annotatedElement : SysML::InterfaceUsage;
		:>> baseType = binaryModularSystemInterfaces meta SysML::Usage;
	}

	metadata def <keyInterface> KeyInterfaceMetadata :> ModularSystemInterfaceMetadata {
		doc /* KeyInterfaceMetadata is the practitioner's name for a modular system interface the program
		 * has chosen to manage: "key interface" is not a statutory term, so a key interface is a modular
		 * system interface first, and every view and check treats it as one. */
	}

	metadata def <technicalStandard> StandardMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = standards meta SysML::Usage;
	}

	metadata def <consensusStandard> ConsensusStandardMetadata :> StandardMetadata {
		:>> baseType = consensusStandards meta SysML::Usage;
	}

	metadata def <conformant> ConformantMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::Usage;
		:>> baseType = conformingElements meta SysML::Usage;
	}

	metadata def <conformsTo> ConformedStandardMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::Usage;
		:>> baseType = conformedStandards meta SysML::Usage;
	}

	metadata def <conformance> StandardConformanceMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::ConnectionDefinition;
		:> annotatedElement : SysML::ConnectionUsage;
		:>> baseType = standardConformances meta SysML::Usage;
	}

	metadata def <mosaObjective> MOSAObjectiveMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = mosaObjectives meta SysML::Usage;
	}

	metadata def <modularityRequirement> ModularityRequirementMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = modularityRequirements meta SysML::Usage;
	}

	metadata def <interfaceRequirement> InterfaceRequirementMetadata :> SemanticMetadata {
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = interfaceRequirements meta SysML::Usage;
	}
}
