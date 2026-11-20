library package NPR7150 {
	doc /* NON-NORMATIVE OpenSysML vocabulary for NASA Procedural Requirements NPR 7150.2D, 'NASA
	 * Software Engineering Requirements' (Office of the Chief Engineer, effective 2022-03-08,
	 * expires 2027-03-08), published in the NASA Online Directives Information System (NODIS) at
	 * https://nodis3.gsfc.nasa.gov/displayDir.cfm?Internal_ID=N_PR_7150_002D_&page_name=main, and
	 * for the software assurance and software safety tasks of NASA-STD-8739.8B, 'Software
	 * Assurance and Software Safety Standard' (Office of Safety and Mission Assurance, approved
	 * 2022-09-08), which states them against the same [SWE-nnn] requirement numbers. Both are
	 * works of the United States Government; the library quotes their text verbatim where they
	 * state a requirement, a task or a definition, with the section, table or requirement
	 * identifier the text comes from. The library is a community extension: it is not endorsed by
	 * NASA and it does not replace the NPR, the Standard, NASA-HDBK-2203 or a Center's
	 * implementing procedures. It lets a project model, in standard SysML v2, the software classes
	 * A to F of Appendix D and the classification of each system and subsystem containing software
	 * (SWE-020), the [SWE-nnn] requirements of chapters 2 to 5, the Requirements Mapping Matrix of
	 * Appendix C (Table 2) with the classes each requirement is invoked for and the authority that
	 * approves its tailoring, the bi-directional traceability of Table 1 (SWE-052), the project's
	 * own Requirements Mapping Matrix with its tailoring (SWE-125), and the software assurance and
	 * software safety tasks of NASA-STD-8739.8B Table 1. `#softwareSystem part fsw { :>>
	 * softwareClass = SoftwareClass::B; }` classifies a system, and `requirement swe013 :
	 * SWERequirements::SWE013 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }`
	 * is one row of the project's matrix. ModelingMetadata is re-exported for status and
	 * rationale.
	 */

	private import ScalarValues::Boolean;
	private import ScalarValues::Integer;
	private import ScalarValues::String;
	private import Metaobjects::SemanticMetadata;
	private import KerML::Root::Element;
	private import Parts::Part;
	private import Parts::parts;
	private import Requirements::RequirementCheck;
	private import Requirements::requirementChecks;
	private import StandardViewDefinitions::*;
	private import Views::asElementTable;
	private import DocumentQueries::*;

	public import ModelingMetadata::*;

	/* Appendix D: software classifications */

	enum def SoftwareClass {
		doc /* The software classes of NPR 7150.2D Appendix D. 'The applicability of requirements in
		 * this directive to specific systems and subsystems containing software is determined
		 * through the use of the NASA-wide software classification structure. These definitions are
		 * based on: (1) usage of the software with or within a NASA system, (2) criticality of the
		 * system to NASA's major programs and projects, (3) extent to which humans depend upon the
		 * system, (4) developmental and operational complexity, and (5) extent of the Agency's
		 * investment. Classes A through E cover engineering-related software. Class F cover
		 * Business and Information Technology Infrastructure Software.' (D.1). 'For a given system
		 * or subsystem, software is expected to be uniquely defined within a single class. If more
		 * than one software class appears to apply, then assign the higher of the classes to the
		 * system/subsystem.' (D.2). Each literal quotes the title and the opening definition of its
		 * class; Appendix D adds examples and exclusions.
		 */
		A {
			doc /* 'Class A: Human Rated Space Software Systems'. 'Human Space Flight Software Systems:
			 * Ground and flight software systems developed or operated by or for NASA needed to
			 * perform a primary mission objective of human space flight and directly interact with
			 * human space flight systems.'
			 */
		}
		B {
			doc /* 'Class B: Non-Human Space Rated Software Systems or Large Scale Aeronautics
			 * Vehicles'. 'Space Systems involve flight and ground software that should perform
			 * reliably to accomplish primary mission objectives or major function(s) in non-human
			 * space rated systems.' 'Aeronautic Vehicles include large scale aeronautic software
			 * systems unique to NASA'.
			 */
		}
		C {
			doc /* 'Class C: Mission Support Software or Aeronautic Vehicles, or Major
			 * Engineering/Research Facility Software'. It covers, among others, 'Flight or ground
			 * software necessary for the science return from a single (non-primary) instrument',
			 * 'Flight or ground software used to analyze or process mission data', non-large scale
			 * aeronautic software systems, systems that operate a major facility for research,
			 * development, test, or evaluation, sounding rockets and their payloads, and 'Software
			 * for NASA Class D payloads, as defined in NPR 8705.4'.
			 */
		}
		D {
			doc /* 'Class D: Basic Science/Engineering Design and Research and Technology Software'.
			 * It covers, among others, 'Ground software that performs secondary science data
			 * analysis', 'Ground software tools that support engineering development' and 'Ground
			 * software tools that support mission planning or formulation'.
			 */
		}
		E {
			doc /* 'Class E: Design Concept, Research, Technology, and General Purpose Software'.
			 * 'Software developed to explore a design concept or hypothesis but not used to make
			 * decisions for an operational Class A, B, or C system or a to-be-built Class A, B, or C
			 * system.' 'Class E software cannot be safety-critical software. If the software is
			 * classified as safety-critical software, then it has to be classified as Class D or
			 * higher.'
			 */
		}
		F {
			doc /* 'Class F: General Purpose Computing, Business, and IT Software'. 'General purpose
			 * computing Business and IT software used in support of the Agency, multiple Centers,
			 * multiple programs/projects, single Centers/projects, or locally deployed General
			 * Purpose Infrastructure To-Be Component of the NASA Enterprise Architecture.'
			 */
		}
	}

	part def SoftwareSystem :> Part {
		doc /* A system or subsystem containing software, the unit NPR 7150.2D classifies: 'The project
		 * manager shall classify each system and subsystem containing software in accordance with
		 * the highest applicable software classification definitions for Classes A, B, C, D, E, and
		 * F software in Appendix D. [SWE-020]' (section 3.5.1). `softwareClass` is that
		 * classification. `safetyCritical` records the determination of section 3.7.1: 'The
		 * project manager, in conjunction with the SMA organization, shall determine if each
		 * software component is considered to be safety-critical per the criteria defined in
		 * NASA-STD-8739.8. [SWE-205]'. NASA-STD-8739.8B section 4.2 states the criteria: 'Software
		 * is classified as safety-critical if it meets at least one of the following criteria: a.
		 * Causes or contributes to a system hazardous condition/event, b. Controls functions
		 * identified in a system hazard, c. Provides mitigation for a system hazardous
		 * condition/event, d. Mitigates damage if a hazardous condition/event occurs, e. Detects,
		 * reports, and takes corrective action if the system reaches a potentially hazardous
		 * state.'
		 */
		attribute softwareClass : SoftwareClass[0..1];
		attribute safetyCritical : Boolean[0..1];
	}

	abstract part softwareSystems : SoftwareSystem[0..*] nonunique :> parts;

	/* Appendix C: the Requirements Mapping Matrix */

	enum def TailoringAuthority {
		doc /* The Authority columns of Table 2: 'The Authority Level responsible for assessing a
		 * project's requirements mapping matrices and any requested tailoring from requirements in
		 * this NPR.' (C.2 d). 'The approval of the Authority designated in Appendix C is required
		 * for all tailoring of requirements designated as "X."' (section 2.2.5)
		 */
		center {
			doc /* 'Center - Center Director or the Center Director's designated ETA, the Center
			 * Director's designated SMA TA, and the CHMO designated for Health and Medical TA.'
			 * (C.3)
			 */
		}
		centerAndCenterCIO {
			doc /* 'Center and Center CIO': the Center authority jointly with the Center CIO, which
			 * Table 2 names for cybersecurity requirements of section 3.11.
			 */
		}
		hqOSMA {
			doc /* 'HQ OSMA': the NASA Headquarters Office of Safety and Mission Assurance, which
			 * Table 2 names for SWE-141.
			 */
		}
		hqOCEAndHQOSMA {
			doc /* 'HQ OCE and HQ OSMA': the NASA Headquarters Office of the Chief Engineer and Office
			 * of Safety and Mission Assurance, which Table 2 names for SWE-032.
			 */
		}
		cio {
			doc /* 'CIO - The NASA CIO, or the designee Center CIO, has institutional authority on all
			 * Class F software projects and has joint responsibility with the ETA on the
			 * cybersecurity requirements in section 3.11, per the direction in the Requirements
			 * Mapping Matrix.' (C.3)
			 */
		}
	}

	metadata def ClassApplicability {
		doc /* One 'X' of Table 2, the Requirements Mapping Matrix: the annotated requirement is invoked
		 * for `softwareClass`. 'X - Indicates an invoked requirement by this NPR consistent with
		 * Software Classification (ref. SWE-139). May be tailored with TA approval (ref. Chapter
		 * 2.2). Blank - Optional/Not invoked by this NPR.' (C.3). The definitions in SWERequirements
		 * carry one annotation per marked cell, so a tool can read the whole table from the model.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		attribute softwareClass : SoftwareClass;
	}

	metadata def SafetyCriticalSoftwareCondition {
		doc /* Marks a requirement the NPR conditions on safety-critical software: sections 3.7.2 to
		 * 3.7.5 open with 'If a project has safety-critical software' (SWE-023, SWE-219, SWE-220)
		 * or 'If a project has safety-critical software or mission-critical software' (SWE-134).
		 */
		:> annotatedElement : SysML::RequirementDefinition;
	}

	enum def TraceabilityRelation {
		doc /* The rows of Table 1, 'Bi-directional traceability by software classification', of section
		 * 3.12.1: the pairs of software elements between which SWE-052 has the project manager
		 * 'perform, record, and maintain bi-directional traceability'.
		 */
		higherLevelRequirementsToSoftwareRequirements {
			doc /* 'Higher-level requirements to the software requirements'. */
		}
		softwareRequirementsToSystemHazards {
			doc /* 'Software requirements to the system hazards'. */
		}
		softwareRequirementsToSoftwareDesignComponents {
			doc /* 'Software requirements to the software design components'. */
		}
		softwareDesignComponentsToSoftwareCode {
			doc /* 'Software design components to the software code'. */
		}
		softwareRequirementsToSoftwareVerifications {
			doc /* 'Software requirements to the software verification(s)'. */
		}
		softwareRequirementsToSoftwareNonConformances {
			doc /* 'Software requirements to the software non-conformances'. */
		}
	}

	metadata def TraceabilityExpectation {
		doc /* One 'X' of Table 1 of section 3.12.1: SWE-052 asks for the bi-directional traceability
		 * `relation` of software of `softwareClass`. Table 1 has the columns 'Class A, B, and C',
		 * 'Class D' and 'Class F'; the first is recorded as one annotation per class. SWE052
		 * carries the annotations.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		attribute relation : TraceabilityRelation;
		attribute softwareClass : SoftwareClass;
	}

	requirement def SWERequirement :> RequirementCheck {
		doc /* One [SWE-nnn] requirement of NPR 7150.2D. The documentation of each definition in
		 * SWERequirements and InstitutionalRequirements is the requirement statement; `id` and
		 * `section` are the 'SWE #' and 'Section' columns of Table 2, and `authority` and
		 * `classFAuthority` its 'Class A-E Authority' and 'Class F Authority' columns. The
		 * rationale of each requirement is in NASA-HDBK-2203 (C.1) and is not reproduced. A
		 * project states its mapping to each requirement by annotating a usage of the definition
		 * with `@RequirementMapping`.
		 */
		attribute id : String;
		attribute section : String;
		attribute authority : TailoringAuthority[0..1];
		attribute classFAuthority : TailoringAuthority[0..1];
	}

	abstract requirement sweRequirements : SWERequirement[0..*] nonunique :> requirementChecks;

	requirement def SATask :> RequirementCheck {
		doc /* One software assurance and software safety task of NASA-STD-8739.8B Table 1. The
		 * documentation of each definition in SoftwareAssurance is the task statement; `sweId` is
		 * the NPR 7150.2D requirement of the row the task belongs to, `sweItem` the lettered item
		 * of that requirement where Table 1 splits a requirement into one row per item, and
		 * `taskNumber` the number of the task in its row. NASA-STD-8739.8B section 4.3 has the
		 * tasks apply by the software classification and the project's Requirements Mapping
		 * Matrix: a task applies where the requirement of its row does.
		 */
		attribute id : String;
		attribute sweId : String;
		attribute sweItem : String[0..1];
		attribute taskNumber : Integer;
	}

	abstract requirement saTasks : SATask[0..*] nonunique :> requirementChecks;

	package SWERequirements {
		doc /* The 100 project-level requirements of NPR 7150.2D chapters 3, 4 and 5 that Table 2 of
		 * Appendix C, the Requirements Mapping Matrix, lists, one definition each, named by their
		 * SWE number and in the order of the table. The documentation of each definition is the
		 * requirement statement of the chapter; each carries one ClassApplicability annotation per
		 * class Table 2 marks 'X' for it, and the Class A-E and Class F authority of its row. Table
		 * 2 marks SWE-015 for Class F without naming a Class F authority, and SWE015 records the
		 * row as printed.
		 */

		requirement def <'SWE-033'> SWE033 :> SWERequirement {
			doc /* The project manager shall assess options for software acquisition versus
			 * development.
			 */
			:>> id = "SWE-033";
			:>> section = "3.1.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-013'> SWE013 :> SWERequirement {
			doc /* The project manager shall develop, maintain, and execute software plans,
			 * including security plans, that cover the entire software life cycle and, as a
			 * minimum, address the requirements of this directive with approved tailoring.
			 */
			:>> id = "SWE-013";
			:>> section = "3.1.3";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-024'> SWE024 :> SWERequirement {
			doc /* The project manager shall track the actual results and performance of software
			 * activities against the software plans.
			 * a. Corrective actions are taken, recorded, and managed to closure.
			 * b. Changes to commitments (e.g., software plans) that have been agreed to by the
			 * affected groups and individuals are taken, recorded, and managed.
			 */
			:>> id = "SWE-024";
			:>> section = "3.1.4";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-034'> SWE034 :> SWERequirement {
			doc /* The project manager shall define and document the acceptance criteria for the
			 * software.
			 */
			:>> id = "SWE-034";
			:>> section = "3.1.5";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-036'> SWE036 :> SWERequirement {
			doc /* The project manager shall establish and maintain the software processes, software
			 * documentation plans, list of developed electronic products, deliverables, and list of
			 * tasks for the software development that are required for the project's software
			 * developers, as well as the action required (e.g., approval, review) of the Government
			 * upon receipt of each of the deliverables.
			 */
			:>> id = "SWE-036";
			:>> section = "3.1.6";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-037'> SWE037 :> SWERequirement {
			doc /* The project manager shall define and document the milestones at which the
			 * software developer(s) progress will be reviewed and audited.
			 */
			:>> id = "SWE-037";
			:>> section = "3.1.7";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-039'> SWE039 :> SWERequirement {
			doc /* The project manager shall require the software developer(s) to periodically
			 * report status and provide insight into software development and test activities; at a
			 * minimum, the software developer(s) will be required to allow the project manager and
			 * software assurance personnel to:
			 * a. Monitor product integration.
			 * b. Review the verification activities to ensure adequacy.
			 * c. Review trade studies and source data.
			 * d. Audit the software development processes and practices.
			 * e. Participate in software reviews and technical interchange meetings.
			 */
			:>> id = "SWE-039";
			:>> section = "3.1.8";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-040'> SWE040 :> SWERequirement {
			doc /* The project manager shall require the software developer(s) to provide NASA with
			 * software products, traceability, software change tracking information, and
			 * nonconformances in electronic format, including software development and management
			 * metrics.
			 */
			:>> id = "SWE-040";
			:>> section = "3.1.9";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-042'> SWE042 :> SWERequirement {
			doc /* The project manager shall require the software developer(s) to provide NASA with
			 * electronic access to the source code developed for the project in a modifiable
			 * format.
			 */
			:>> id = "SWE-042";
			:>> section = "3.1.10";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-139'> SWE139 :> SWERequirement {
			doc /* The project manager shall comply with the requirements in this NPR that are
			 * marked with an 'X' in Appendix C consistent with their software classification.
			 */
			:>> id = "SWE-139";
			:>> section = "3.1.11";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-121'> SWE121 :> SWERequirement {
			doc /* Where approved, the project manager shall document and reflect the tailored
			 * requirement in the plans or procedures controlling the development, acquisition, and
			 * deployment of the affected software.
			 */
			:>> id = "SWE-121";
			:>> section = "3.1.12";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-125'> SWE125 :> SWERequirement {
			doc /* Each project manager with software components shall maintain a requirements
			 * mapping matrix or multiple requirements mapping matrices against requirements in this
			 * NPR, including those delegated to other parties or accomplished by contract vehicles
			 * or Space Act Agreements.
			 */
			:>> id = "SWE-125";
			:>> section = "3.1.13";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-027'> SWE027 :> SWERequirement {
			doc /* The project manager shall satisfy the following conditions when a COTS, GOTS,
			 * MOTS, OSS, or reused software component is acquired or used:
			 * a. The requirements to be met by the software component are identified.
			 * b. The software component includes documentation to fulfill its intended purpose
			 * (e.g., usage instructions).
			 * c. Proprietary rights, usage rights, ownership, warranty, licensing rights, transfer
			 * rights, and conditions of use (e.g., required copyright, author, and applicable
			 * license notices within the software code, or a requirement to redistribute the
			 * licensed software only under the same license (e.g., GNU GPL, ver. 3, license)) have
			 * been addressed and coordinated with Center Intellectual Property Counsel.
			 * d. Future support for the software product is planned and adequate for project needs.
			 * e. The software component is verified and validated to the same level required to
			 * accept a similar developed software component for its intended use.
			 * f. The project has a plan to perform periodic assessments of vendor reported defects
			 * to ensure the defects do not impact the selected software components.
			 */
			:>> id = "SWE-027";
			:>> section = "3.1.14";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-015'> SWE015 :> SWERequirement {
			doc /* To better estimate the cost of development, the project manager shall establish,
			 * document, and maintain:
			 * a. Two cost estimate models and associated cost parameters for all Class A and B
			 * software projects that have an estimated project cost of $2 million or more.
			 * b. One software cost estimate model and associated cost parameter(s) for all Class A
			 * and Class B software projects that have an estimated project cost of less than $2
			 * million.
			 * c. One software cost estimate model and associated cost parameter(s) for all Class C
			 * and Class D software projects.
			 * d. One software cost estimate model and associated cost parameter(s) for all Class F
			 * software projects.
			 */
			:>> id = "SWE-015";
			:>> section = "3.2.1";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-151'> SWE151 :> SWERequirement {
			doc /* The project manager's software cost estimate(s) shall satisfy the following
			 * conditions:
			 * a. Covers the entire software life cycle.
			 * b. Is based on selected project attributes (e.g., programmatic
			 * assumptions/constraints, assessment of the size, functionality, complexity,
			 * criticality, reuse code, modified code, and risk of the software processes and
			 * products).
			 * c. Is based on the cost implications of the technology to be used and the required
			 * maturation of that technology.
			 * d. Incorporates risk and uncertainty, including end state risk and threat assessments
			 * for cybersecurity.
			 * e. Includes the cost of the required software assurance support.
			 * f. Includes other direct costs.
			 */
			:>> id = "SWE-151";
			:>> section = "3.2.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-174'> SWE174 :> SWERequirement {
			doc /* The project manager shall submit software planning parameters, including size and
			 * effort estimates, milestones, and characteristics, to the Center measurement
			 * repository at the conclusion of major milestones.
			 */
			:>> id = "SWE-174";
			:>> section = "3.2.3";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
		}

		requirement def <'SWE-016'> SWE016 :> SWERequirement {
			doc /* The project manager shall document and maintain a software schedule that
			 * satisfies the following conditions:
			 * a. Coordinates with the overall project schedule.
			 * b. Documents the interactions of milestones and deliverables between software,
			 * hardware, operations, and the rest of the system.
			 * c. Reflects the critical dependencies for software development activities.
			 * d. Identifies and accounts for dependencies with other projects and cross-program
			 * dependencies.
			 */
			:>> id = "SWE-016";
			:>> section = "3.3.1";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-018'> SWE018 :> SWERequirement {
			doc /* The project manager shall regularly hold reviews of software schedule activities,
			 * status, performance metrics, and assessment/analysis results with the project
			 * stakeholders and track issues to resolution.
			 */
			:>> id = "SWE-018";
			:>> section = "3.3.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-046'> SWE046 :> SWERequirement {
			doc /* The project manager shall require the software developer(s) to provide a software
			 * schedule for the project's review, and schedule updates as requested.
			 */
			:>> id = "SWE-046";
			:>> section = "3.3.3";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-017'> SWE017 :> SWERequirement {
			doc /* The project manager shall plan, track, and ensure project specific software
			 * training for project personnel.
			 */
			:>> id = "SWE-017";
			:>> section = "3.4.1";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-020'> SWE020 :> SWERequirement {
			doc /* The project manager shall classify each system and subsystem containing software
			 * in accordance with the highest applicable software classification definitions for
			 * Classes A, B, C, D, E, and F software in Appendix D.
			 */
			:>> id = "SWE-020";
			:>> section = "3.5.1";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-176'> SWE176 :> SWERequirement {
			doc /* The project manager shall maintain records of each software classification
			 * determination, each software Requirements Mapping Matrix, and the results of each
			 * software independent classification assessments for the life of the project.
			 */
			:>> id = "SWE-176";
			:>> section = "3.5.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-022'> SWE022 :> SWERequirement {
			doc /* The project manager shall plan and implement software assurance, software safety,
			 * and IV&V (if required) per NASA-STD-8739.8, Software Assurance and Software Safety
			 * Standard.
			 */
			:>> id = "SWE-022";
			:>> section = "3.6.1";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
		}

		requirement def <'SWE-141'> SWE141 :> SWERequirement {
			doc /* For projects reaching Key Decision Point A, the program manager shall ensure that
			 * software IV&V is performed on the following categories of projects:
			 * a. Category 1 projects as defined in NPR 7120.5.
			 * b. Category 2 projects as defined in NPR 7120.5, that have Class A or Class B payload
			 * risk classification per NPR 8705.4, Risk Classification for NASA Payloads.
			 * c. Projects selected explicitly by the Mission Directorate Associate Administrator
			 * (MDAA) to have software IV&V.
			 */
			:>> id = "SWE-141";
			:>> section = "3.6.2";
			:>> authority = TailoringAuthority::hqOSMA;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
		}

		requirement def <'SWE-131'> SWE131 :> SWERequirement {
			doc /* If software IV&V is required for a project, the project manager, in consultation
			 * with NASA IV&V, shall ensure an IPEP is developed, approved, maintained, and executed
			 * in accordance with IV&V requirements in NASA-STD-8739.8.
			 */
			:>> id = "SWE-131";
			:>> section = "3.6.3";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
		}

		requirement def <'SWE-178'> SWE178 :> SWERequirement {
			doc /* If software IV&V is performed on a project, the project manager shall ensure that
			 * IV&V is provided access to development artifacts, products, source code, and data
			 * required to perform the IV&V analysis efficiently and effectively.
			 */
			:>> id = "SWE-178";
			:>> section = "3.6.4";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
		}

		requirement def <'SWE-179'> SWE179 :> SWERequirement {
			doc /* If software IV&V is performed on a project, the project manager shall provide
			 * responses to IV&V submitted issues and risks and track these issues and risks to
			 * closure.
			 */
			:>> id = "SWE-179";
			:>> section = "3.6.5";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
		}

		requirement def <'SWE-205'> SWE205 :> SWERequirement {
			doc /* The project manager, in conjunction with the SMA organization, shall determine if
			 * each software component is considered to be safety-critical per the criteria defined
			 * in NASA-STD-8739.8.
			 */
			:>> id = "SWE-205";
			:>> section = "3.7.1";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
		}

		requirement def <'SWE-023'> SWE023 :> SWERequirement {
			doc /* If a project has safety-critical software, the project manager shall implement
			 * the safety-critical software requirements contained in NASA-STD-8739.8.
			 */
			:>> id = "SWE-023";
			:>> section = "3.7.2";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@SafetyCriticalSoftwareCondition;
		}

		requirement def <'SWE-134'> SWE134 :> SWERequirement {
			doc /* If a project has safety-critical software or mission-critical software, the
			 * project manager shall implement the following items in the software:
			 * a. The software is initialized, at first start and restarts, to a known safe state.
			 * b. The software safely transitions between all predefined known states.
			 * c. Termination performed by software functions is performed to a known safe state.
			 * d. Operator overrides of software functions require at least two independent actions
			 * by an operator.
			 * e. Software rejects commands received out of sequence when execution of those
			 * commands out of sequence can cause a hazard.
			 * f. The software detects inadvertent memory modification and recovers to a known safe
			 * state.
			 * g. The software performs integrity checks on inputs and outputs to/from the software
			 * system.
			 * h. The software performs prerequisite checks prior to the execution of
			 * safety-critical software commands.
			 * i. No single software event or action is allowed to initiate an identified hazard.
			 * j. The software responds to an off-nominal condition within the time needed to
			 * prevent a hazardous event.
			 * k. The software provides error handling.
			 * l. The software can place the system into a safe state.
			 */
			:>> id = "SWE-134";
			:>> section = "3.7.3";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@SafetyCriticalSoftwareCondition;
		}

		requirement def <'SWE-219'> SWE219 :> SWERequirement {
			doc /* If a project has safety-critical software, the project manager shall ensure that
			 * there is 100 percent code test coverage using the Modified Condition/Decision
			 * Coverage (MC/DC) criterion for all identified safety-critical software components.
			 */
			:>> id = "SWE-219";
			:>> section = "3.7.4";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@SafetyCriticalSoftwareCondition;
		}

		requirement def <'SWE-220'> SWE220 :> SWERequirement {
			doc /* If a project has safety-critical software, the project manager shall ensure all
			 * identified safety-critical software components have a cyclomatic complexity value of
			 * 15 or lower. Any exceedance shall be reviewed and waived with rationale by the
			 * project manager or technical approval authority.
			 */
			:>> id = "SWE-220";
			:>> section = "3.7.5";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@SafetyCriticalSoftwareCondition;
		}

		requirement def <'SWE-146'> SWE146 :> SWERequirement {
			doc /* The project manager shall define the approach to the automatic generation of
			 * software source code including:
			 * a. Validation and verification of auto-generation tools.
			 * b. Configuration management of the auto-generation tools and associated data.
			 * c. Description of the limits and the allowable scope for the use of the
			 * auto-generated software.
			 * d. Verification and validation of auto-generated source code using the same software
			 * standards and processes as hand-generated code.
			 * e. Monitoring the actual use of auto-generated source code compared to the planned
			 * use.
			 * f. Policies and procedures for making manual changes to auto-generated source code.
			 * g. Configuration management of the input to the auto-generation tool, the output of
			 * the auto-generation tool, and modifications made to the output of the auto-generation
			 * tools.
			 */
			:>> id = "SWE-146";
			:>> section = "3.8.1";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-206'> SWE206 :> SWERequirement {
			doc /* The project manager shall require the software developers and custom software
			 * suppliers to provide NASA with electronic access to the models, simulations, and
			 * associated data used as inputs for auto-generation of software.
			 */
			:>> id = "SWE-206";
			:>> section = "3.8.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-032'> SWE032 :> SWERequirement {
			doc /* The project manager shall acquire, develop, and maintain software from an
			 * organization with a non-expired CMMI(R)-DEV rating as measured by a CMMI(R) Institute
			 * Certified Lead Appraiser as follows:
			 * a. For Class A software: CMMI(R)-DEV Maturity Level 3 Rating or higher for software.
			 * b. For Class B software (except Class B software on NASA Class D payloads, as defined
			 * in NPR 8705.4): CMMI(R)-DEV Maturity Level 2 Rating or higher for software.
			 */
			:>> id = "SWE-032";
			:>> section = "3.9.2";
			:>> authority = TailoringAuthority::hqOCEAndHQOSMA;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
		}

		requirement def <'SWE-147'> SWE147 :> SWERequirement {
			doc /* The project manager shall specify reusability requirements that apply to its
			 * software development activities to enable future reuse of the software, including the
			 * models, simulations, and associated data used as inputs for auto-generation of
			 * software, for U.S. Government purposes.
			 */
			:>> id = "SWE-147";
			:>> section = "3.10.1";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-148'> SWE148 :> SWERequirement {
			doc /* The project manager shall evaluate software for potential reuse by other projects
			 * across NASA and contribute reuse candidates to the appropriate NASA internal sharing
			 * and reuse software system. However, if the project manager is not a civil servant,
			 * then a civil servant will pre-approve all such software contributions; all software
			 * contributions should include, at a minimum, the following information:
			 * a. Software Title.
			 * b. Software Description.
			 * c. The Civil Servant Software Technical POC for the software product.
			 * d. The language or languages used to develop the software.
			 * e. Any third-party code contained therein, and the record of the requisite license or
			 * permission received from the third party permitting the Government's use and any
			 * required markings (e.g., required copyright, author, applicable license notices
			 * within the software code, and the source of each third-party software component
			 * (e.g., software URL & license URL)), if applicable.
			 * f. Release notes.
			 */
			:>> id = "SWE-148";
			:>> section = "3.10.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-156'> SWE156 :> SWERequirement {
			doc /* The project manager shall perform a software cybersecurity assessment on the
			 * software components per the Agency security policies and the project requirements,
			 * including risks posed by the use of COTS, GOTS, MOTS, OSS, or reused software
			 * components.
			 */
			:>> id = "SWE-156";
			:>> section = "3.11.2";
			:>> authority = TailoringAuthority::centerAndCenterCIO;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::E; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-154'> SWE154 :> SWERequirement {
			doc /* The project manager shall identify cybersecurity risks, along with their
			 * mitigations, in flight and ground software systems and plan the mitigations for these
			 * systems.
			 */
			:>> id = "SWE-154";
			:>> section = "3.11.3";
			:>> authority = TailoringAuthority::centerAndCenterCIO;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
		}

		requirement def <'SWE-157'> SWE157 :> SWERequirement {
			doc /* The project manager shall implement protections for software systems with
			 * communications capabilities against unauthorized access per the requirements
			 * contained in the NASA-STD-1006, Space System Protection Standard.
			 */
			:>> id = "SWE-157";
			:>> section = "3.11.4";
			:>> authority = TailoringAuthority::centerAndCenterCIO;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
		}

		requirement def <'SWE-159'> SWE159 :> SWERequirement {
			doc /* The project manager shall test the software and record test results for the
			 * required software cybersecurity mitigation implementations identified from the
			 * security vulnerabilities and security weaknesses analysis.
			 */
			:>> id = "SWE-159";
			:>> section = "3.11.5";
			:>> authority = TailoringAuthority::centerAndCenterCIO;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-207'> SWE207 :> SWERequirement {
			doc /* The project manager shall identify, record, and implement secure coding
			 * practices.
			 */
			:>> id = "SWE-207";
			:>> section = "3.11.6";
			:>> authority = TailoringAuthority::centerAndCenterCIO;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
		}

		requirement def <'SWE-185'> SWE185 :> SWERequirement {
			doc /* The project manager shall verify that the software code meets the project's
			 * secure coding standard by using the results from static analysis tool(s).
			 */
			:>> id = "SWE-185";
			:>> section = "3.11.7";
			:>> authority = TailoringAuthority::centerAndCenterCIO;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-210'> SWE210 :> SWERequirement {
			doc /* The project manager shall identify software requirements for the collection,
			 * reporting, and storage of data relating to the detection of adversarial actions.
			 */
			:>> id = "SWE-210";
			:>> section = "3.11.8";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-052'> SWE052 :> SWERequirement {
			doc /* The project manager shall perform, record, and maintain bi-directional
			 * traceability between the following software elements:
			 */
			:>> id = "SWE-052";
			:>> section = "3.12.1";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
			@TraceabilityExpectation { relation = TraceabilityRelation::higherLevelRequirementsToSoftwareRequirements; softwareClass = SoftwareClass::A; }
			@TraceabilityExpectation { relation = TraceabilityRelation::higherLevelRequirementsToSoftwareRequirements; softwareClass = SoftwareClass::B; }
			@TraceabilityExpectation { relation = TraceabilityRelation::higherLevelRequirementsToSoftwareRequirements; softwareClass = SoftwareClass::C; }
			@TraceabilityExpectation { relation = TraceabilityRelation::higherLevelRequirementsToSoftwareRequirements; softwareClass = SoftwareClass::F; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSystemHazards; softwareClass = SoftwareClass::A; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSystemHazards; softwareClass = SoftwareClass::B; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSystemHazards; softwareClass = SoftwareClass::C; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSystemHazards; softwareClass = SoftwareClass::D; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareDesignComponents; softwareClass = SoftwareClass::A; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareDesignComponents; softwareClass = SoftwareClass::B; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareDesignComponents; softwareClass = SoftwareClass::C; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareDesignComponentsToSoftwareCode; softwareClass = SoftwareClass::A; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareDesignComponentsToSoftwareCode; softwareClass = SoftwareClass::B; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareDesignComponentsToSoftwareCode; softwareClass = SoftwareClass::C; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareVerifications; softwareClass = SoftwareClass::A; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareVerifications; softwareClass = SoftwareClass::B; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareVerifications; softwareClass = SoftwareClass::C; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareVerifications; softwareClass = SoftwareClass::D; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareVerifications; softwareClass = SoftwareClass::F; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareNonConformances; softwareClass = SoftwareClass::A; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareNonConformances; softwareClass = SoftwareClass::B; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareNonConformances; softwareClass = SoftwareClass::C; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareNonConformances; softwareClass = SoftwareClass::D; }
			@TraceabilityExpectation { relation = TraceabilityRelation::softwareRequirementsToSoftwareNonConformances; softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-050'> SWE050 :> SWERequirement {
			doc /* The project manager shall establish, capture, record, approve, and maintain
			 * software requirements, including requirements for COTS, GOTS, MOTS, OSS, or reused
			 * software components, as part of the technical specification.
			 */
			:>> id = "SWE-050";
			:>> section = "4.1.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-051'> SWE051 :> SWERequirement {
			doc /* The project manager shall perform software requirements analysis based on
			 * flowed-down and derived requirements from the top-level systems engineering
			 * requirements, safety and reliability analyses, and the hardware specifications and
			 * design.
			 */
			:>> id = "SWE-051";
			:>> section = "4.1.3";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-184'> SWE184 :> SWERequirement {
			doc /* The project manager shall include software related safety constraints, controls,
			 * mitigations, and assumptions between the hardware, operator, and software in the
			 * software requirements documentation.
			 */
			:>> id = "SWE-184";
			:>> section = "4.1.4";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-053'> SWE053 :> SWERequirement {
			doc /* The project manager shall track and manage changes to the software requirements.
			 */
			:>> id = "SWE-053";
			:>> section = "4.1.5";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-054'> SWE054 :> SWERequirement {
			doc /* The project manager shall identify, initiate corrective actions, and track until
			 * closure inconsistencies among requirements, project plans, and software products.
			 */
			:>> id = "SWE-054";
			:>> section = "4.1.6";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-055'> SWE055 :> SWERequirement {
			doc /* The project manager shall perform requirements validation to ensure that the
			 * software will perform as intended in the customer environment.
			 */
			:>> id = "SWE-055";
			:>> section = "4.1.7";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-057'> SWE057 :> SWERequirement {
			doc /* The project manager shall transform the requirements for the software into a
			 * recorded software architecture.
			 */
			:>> id = "SWE-057";
			:>> section = "4.2.3";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-143'> SWE143 :> SWERequirement {
			doc /* The project manager shall perform a software architecture review on the following
			 * categories of projects:
			 * a. Category 1 Projects as defined in NPR 7120.5.
			 * b. Category 2 Projects as defined in NPR 7120.5, that have Class A or Class B payload
			 * risk classification per NPR 8705.4.
			 */
			:>> id = "SWE-143";
			:>> section = "4.2.4";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-058'> SWE058 :> SWERequirement {
			doc /* The project manager shall develop, record, and maintain a software design based
			 * on the software architectural design that describes the lower-level units so that
			 * they can be coded, compiled, and tested.
			 */
			:>> id = "SWE-058";
			:>> section = "4.3.2";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-060'> SWE060 :> SWERequirement {
			doc /* The project manager shall implement the software design into software code.
			 */
			:>> id = "SWE-060";
			:>> section = "4.4.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-061'> SWE061 :> SWERequirement {
			doc /* The project manager shall select, define, and adhere to software coding methods,
			 * standards, and criteria.
			 */
			:>> id = "SWE-061";
			:>> section = "4.4.3";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-135'> SWE135 :> SWERequirement {
			doc /* The project manager shall use static analysis tools to analyze the code during
			 * the development and testing phases to, at a minimum, detect defects, software
			 * security, code coverage, and software complexity.
			 */
			:>> id = "SWE-135";
			:>> section = "4.4.4";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-062'> SWE062 :> SWERequirement {
			doc /* The project manager shall unit test the software code.
			 */
			:>> id = "SWE-062";
			:>> section = "4.4.5";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-186'> SWE186 :> SWERequirement {
			doc /* The project manager shall assure that the unit test results are repeatable.
			 */
			:>> id = "SWE-186";
			:>> section = "4.4.6";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-063'> SWE063 :> SWERequirement {
			doc /* The project manager shall provide a software version description for each
			 * software release.
			 */
			:>> id = "SWE-063";
			:>> section = "4.4.7";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-136'> SWE136 :> SWERequirement {
			doc /* The project manager shall validate and accredit the software tool(s) required to
			 * develop or maintain software.
			 */
			:>> id = "SWE-136";
			:>> section = "4.4.8";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-065'> SWE065 :> SWERequirement {
			doc /* The project manager shall establish and maintain:
			 * a. Software test plan(s).
			 * b. Software test procedure(s).
			 * c. Software test(s), including any code specifically written to perform test
			 * procedures.
			 * d. Software test report(s).
			 */
			:>> id = "SWE-065";
			:>> section = "4.5.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-066'> SWE066 :> SWERequirement {
			doc /* The project manager shall test the software against its requirements.
			 */
			:>> id = "SWE-066";
			:>> section = "4.5.3";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-187'> SWE187 :> SWERequirement {
			doc /* The project manager shall place software items under configuration management
			 * prior to testing.
			 */
			:>> id = "SWE-187";
			:>> section = "4.5.4";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-068'> SWE068 :> SWERequirement {
			doc /* The project manager shall evaluate test results and record the evaluation.
			 */
			:>> id = "SWE-068";
			:>> section = "4.5.5";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-070'> SWE070 :> SWERequirement {
			doc /* The project manager shall use validated and accredited software models,
			 * simulations, and analysis tools required to perform qualification of flight software
			 * or flight equipment.
			 */
			:>> id = "SWE-070";
			:>> section = "4.5.6";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-071'> SWE071 :> SWERequirement {
			doc /* The project manager shall update the software test and verification plan(s) and
			 * procedure(s) to be consistent with software requirements.
			 */
			:>> id = "SWE-071";
			:>> section = "4.5.7";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-073'> SWE073 :> SWERequirement {
			doc /* The project manager shall validate the software system on the targeted platform
			 * or high-fidelity simulation.
			 */
			:>> id = "SWE-073";
			:>> section = "4.5.8";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-189'> SWE189 :> SWERequirement {
			doc /* The project manager shall ensure that the code coverage measurements for the
			 * software are selected, implemented, tracked, recorded, and reported.
			 */
			:>> id = "SWE-189";
			:>> section = "4.5.9";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
		}

		requirement def <'SWE-190'> SWE190 :> SWERequirement {
			doc /* The project manager shall verify code coverage is measured by analysis of the
			 * results of the execution of tests.
			 */
			:>> id = "SWE-190";
			:>> section = "4.5.10";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-191'> SWE191 :> SWERequirement {
			doc /* The project manager shall plan and conduct software regression testing to
			 * demonstrate that defects have not been introduced into previously integrated or
			 * tested software and have not produced a security vulnerability.
			 */
			:>> id = "SWE-191";
			:>> section = "4.5.11";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-192'> SWE192 :> SWERequirement {
			doc /* The project manager shall verify through test the software requirements that
			 * trace to a hazardous event, cause, or mitigation technique.
			 */
			:>> id = "SWE-192";
			:>> section = "4.5.12";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
		}

		requirement def <'SWE-193'> SWE193 :> SWERequirement {
			doc /* The project manager shall develop acceptance tests for loaded or uplinked data,
			 * rules, and code that affects software and software system behavior.
			 */
			:>> id = "SWE-193";
			:>> section = "4.5.13";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-211'> SWE211 :> SWERequirement {
			doc /* The project manager shall test embedded COTS, GOTS, MOTS, OSS, or reused software
			 * components to the same level required to accept a custom developed software component
			 * for its intended use.
			 */
			:>> id = "SWE-211";
			:>> section = "4.5.14";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-075'> SWE075 :> SWERequirement {
			doc /* The project manager shall plan and implement software operations, maintenance,
			 * and retirement activities.
			 */
			:>> id = "SWE-075";
			:>> section = "4.6.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-077'> SWE077 :> SWERequirement {
			doc /* The project manager shall complete and deliver the software product to the
			 * customer with appropriate records, including as-built records, to support the
			 * operations and maintenance phase of the software's life cycle.
			 */
			:>> id = "SWE-077";
			:>> section = "4.6.3";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-194'> SWE194 :> SWERequirement {
			doc /* The project manager shall complete, prior to delivery, verification that all
			 * software requirements identified for this delivery have been met or dispositioned,
			 * that all approved changes have been implemented, and that all defects designated for
			 * resolution prior to delivery have been resolved.
			 */
			:>> id = "SWE-194";
			:>> section = "4.6.4";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-195'> SWE195 :> SWERequirement {
			doc /* The project manager shall maintain the software using standards and processes per
			 * the applicable software classification throughout the maintenance phase.
			 */
			:>> id = "SWE-195";
			:>> section = "4.6.5";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-196'> SWE196 :> SWERequirement {
			doc /* The project manager shall identify the records and software tools to be archived,
			 * the location of the archive, and procedures for access to the products for software
			 * retirement or disposal.
			 */
			:>> id = "SWE-196";
			:>> section = "4.6.6";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-079'> SWE079 :> SWERequirement {
			doc /* The project manager shall develop a software configuration management plan that
			 * describes the functions, responsibilities, and authority for the implementation of
			 * software configuration management for the project.
			 */
			:>> id = "SWE-079";
			:>> section = "5.1.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-080'> SWE080 :> SWERequirement {
			doc /* The project manager shall track and evaluate changes to software products.
			 */
			:>> id = "SWE-080";
			:>> section = "5.1.3";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-081'> SWE081 :> SWERequirement {
			doc /* The project manager shall identify the software configuration items (e.g.,
			 * software records, code, data, tools, models, scripts) and their versions to be
			 * controlled for the project.
			 */
			:>> id = "SWE-081";
			:>> section = "5.1.4";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-082'> SWE082 :> SWERequirement {
			doc /* The project manager shall establish and implement procedures to:
			 * a. Designate the levels of control through which each identified software
			 * configuration item is required to pass.
			 * b. Identify the persons or groups with authority to authorize changes.
			 * c. Identify the persons or groups to make changes at each level.
			 */
			:>> id = "SWE-082";
			:>> section = "5.1.5";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-083'> SWE083 :> SWERequirement {
			doc /* The project manager shall prepare and maintain records of the configuration
			 * status of software configuration items.
			 */
			:>> id = "SWE-083";
			:>> section = "5.1.6";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-084'> SWE084 :> SWERequirement {
			doc /* The project manager shall perform software configuration audits to determine the
			 * correct version of the software configuration items and verify that they conform to
			 * the records that define them.
			 */
			:>> id = "SWE-084";
			:>> section = "5.1.7";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-085'> SWE085 :> SWERequirement {
			doc /* The project manager shall establish and implement procedures for the storage,
			 * handling, delivery, release, and maintenance of deliverable software products.
			 */
			:>> id = "SWE-085";
			:>> section = "5.1.8";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-045'> SWE045 :> SWERequirement {
			doc /* The project manager shall participate in any joint NASA/developer audits.
			 */
			:>> id = "SWE-045";
			:>> section = "5.1.9";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-086'> SWE086 :> SWERequirement {
			doc /* The project manager shall record, analyze, plan, track, control, and communicate
			 * all of the software risks and mitigation plans.
			 */
			:>> id = "SWE-086";
			:>> section = "5.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-087'> SWE087 :> SWERequirement {
			doc /* The project manager shall perform and report the results of software peer reviews
			 * or software inspections for:
			 * a. Software requirements.
			 * b. Software plans, including cybersecurity.
			 * c. Any design items that the project identified for software peer review or software
			 * inspections according to the software development plans.
			 * d. Software code as defined in the software and or project plans.
			 * e. Software test procedures.
			 */
			:>> id = "SWE-087";
			:>> section = "5.3.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-088'> SWE088 :> SWERequirement {
			doc /* The project manager shall, for each planned software peer review or software
			 * inspection:
			 * a. Use a checklist or formal reading technique (e.g., perspective-based reading) to
			 * evaluate the work products.
			 * b. Use established readiness and completion criteria.
			 * c. Track actions identified in the reviews until they are resolved.
			 * d. Identify the required participants.
			 */
			:>> id = "SWE-088";
			:>> section = "5.3.3";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-089'> SWE089 :> SWERequirement {
			doc /* The project manager shall, for each planned software peer review or software
			 * inspection, record necessary measurements.
			 */
			:>> id = "SWE-089";
			:>> section = "5.3.4";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-090'> SWE090 :> SWERequirement {
			doc /* The project manager shall establish, record, maintain, report, and utilize
			 * software management and technical measurements.
			 */
			:>> id = "SWE-090";
			:>> section = "5.4.2";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-093'> SWE093 :> SWERequirement {
			doc /* The project manager shall analyze software measurement data collected using
			 * documented project-specified and Center/organizational analysis procedures.
			 */
			:>> id = "SWE-093";
			:>> section = "5.4.3";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-094'> SWE094 :> SWERequirement {
			doc /* The project manager shall provide access to the software measurement data,
			 * measurement analyses, and software development status as requested to the sponsoring
			 * Mission Directorate, the NASA Chief Engineer, the Center Technical Authorities, HQ
			 * SMA, and other organizations as appropriate.
			 */
			:>> id = "SWE-094";
			:>> section = "5.4.4";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-199'> SWE199 :> SWERequirement {
			doc /* The project manager shall monitor measures to ensure the software will meet or
			 * exceed performance and functionality requirements, including satisfying constraints.
			 */
			:>> id = "SWE-199";
			:>> section = "5.4.5";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-200'> SWE200 :> SWERequirement {
			doc /* The project manager shall collect, track, and report software requirements
			 * volatility metrics.
			 */
			:>> id = "SWE-200";
			:>> section = "5.4.6";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
		}

		requirement def <'SWE-201'> SWE201 :> SWERequirement {
			doc /* The project manager shall track and maintain software non-conformances (including
			 * defects in tools and appropriate ground software).
			 */
			:>> id = "SWE-201";
			:>> section = "5.5.1";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::D; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-202'> SWE202 :> SWERequirement {
			doc /* The project manager shall define and implement clear software severity levels for
			 * all software non-conformances (including tools, COTS, GOTS, MOTS, OSS, reused
			 * software components, and applicable ground systems).
			 */
			:>> id = "SWE-202";
			:>> section = "5.5.2";
			:>> authority = TailoringAuthority::center;
			:>> classFAuthority = TailoringAuthority::cio;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
			@ClassApplicability { softwareClass = SoftwareClass::F; }
		}

		requirement def <'SWE-203'> SWE203 :> SWERequirement {
			doc /* The project manager shall implement mandatory assessments of reported
			 * non-conformances for all COTS, GOTS, MOTS, OSS, and/or reused software components.
			 */
			:>> id = "SWE-203";
			:>> section = "5.5.3";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
			@ClassApplicability { softwareClass = SoftwareClass::C; }
		}

		requirement def <'SWE-204'> SWE204 :> SWERequirement {
			doc /* The project manager shall implement process assessments for all high-severity
			 * software non-conformances (closed loop process).
			 */
			:>> id = "SWE-204";
			:>> section = "5.5.4";
			:>> authority = TailoringAuthority::center;
			@ClassApplicability { softwareClass = SoftwareClass::A; }
			@ClassApplicability { softwareClass = SoftwareClass::B; }
		}
	}

	package InstitutionalRequirements {
		doc /* The 30 institutional requirements of NPR 7150.2D chapter 2, which the NPR places on
		 * the NASA Office of the Chief Engineer, the NASA Chief, Safety and Mission Assurance, the
		 * NASA Chief Information Officer, Center Directors, the Technical Authorities and others
		 * rather than on the project manager. Table 2 of Appendix C does not list them, so they
		 * carry no ClassApplicability annotation and no authority.
		 */

		requirement def <'SWE-002'> SWE002 :> SWERequirement {
			doc /* The NASA OCE shall lead and maintain a NASA Software Engineering Initiative to
			 * advance software engineering practices.
			 */
			:>> id = "SWE-002";
			:>> section = "2.1.1.1";
		}

		requirement def <'SWE-004'> SWE004 :> SWERequirement {
			doc /* The NASA OCE shall periodically benchmark each Center's software engineering
			 * capability against requirements in this directive.
			 */
			:>> id = "SWE-004";
			:>> section = "2.1.1.2";
		}

		requirement def <'SWE-152'> SWE152 :> SWERequirement {
			doc /* The NASA OCE shall periodically review the project requirements mapping matrices.
			 */
			:>> id = "SWE-152";
			:>> section = "2.1.1.3";
		}

		requirement def <'SWE-129'> SWE129 :> SWERequirement {
			doc /* The NASA OCE shall authorize appraisals against selected requirements in this NPR
			 * to check compliance.
			 */
			:>> id = "SWE-129";
			:>> section = "2.1.1.4";
		}

		requirement def <'SWE-100'> SWE100 :> SWERequirement {
			doc /* The NASA OCE and Center training organizations shall provide training to advance
			 * software engineering practices.
			 */
			:>> id = "SWE-100";
			:>> section = "2.1.1.5";
		}

		requirement def <'SWE-098'> SWE098 :> SWERequirement {
			doc /* The NASA OCE shall maintain an Agency-wide process asset library of applicable
			 * best practices and process templates for all size projects.
			 */
			:>> id = "SWE-098";
			:>> section = "2.1.1.6";
		}

		requirement def <'SWE-208'> SWE208 :> SWERequirement {
			doc /* The NASA Chief, SMA shall lead and maintain a NASA Software Assurance and
			 * Software Safety Initiative to advance software assurance and software safety
			 * practices.
			 */
			:>> id = "SWE-208";
			:>> section = "2.1.2.2";
		}

		requirement def <'SWE-209'> SWE209 :> SWERequirement {
			doc /* The NASA Chief, SMA shall periodically benchmark each Center's software assurance
			 * and software safety capabilities against the NASA-STD-8739.8, NASA Software Assurance
			 * and Software Safety Standard.
			 */
			:>> id = "SWE-209";
			:>> section = "2.1.2.3";
		}

		requirement def <'SWE-212'> SWE212 :> SWERequirement {
			doc /* The NASA Chief, SMA shall periodically review the project's requirements mapping
			 * matrices.
			 */
			:>> id = "SWE-212";
			:>> section = "2.1.2.4";
		}

		requirement def <'SWE-221'> SWE221 :> SWERequirement {
			doc /* The NASA Chief, SMA shall authorize appraisals against selected requirements in
			 * this NPR to check compliance.
			 */
			:>> id = "SWE-221";
			:>> section = "2.1.2.5";
		}

		requirement def <'SWE-222'> SWE222 :> SWERequirement {
			doc /* The NASA Chief, SMA shall provide for software assurance training.
			 */
			:>> id = "SWE-222";
			:>> section = "2.1.2.6";
		}

		requirement def <'SWE-223'> SWE223 :> SWERequirement {
			doc /* The NASA Chief, SMA shall make the final decision on all proposed tailoring of
			 * SWE-141, the Independent Verification and Validation (IV&V) requirement.
			 */
			:>> id = "SWE-223";
			:>> section = "2.1.2.7";
		}

		requirement def <'SWE-003'> SWE003 :> SWERequirement {
			doc /* Center Director, or designee, shall maintain, staff, and implement a plan to
			 * continually advance the Center's in-house software engineering capability and monitor
			 * the software engineering capability of NASA's contractors.
			 */
			:>> id = "SWE-003";
			:>> section = "2.1.5.2";
		}

		requirement def <'SWE-005'> SWE005 :> SWERequirement {
			doc /* Center Director, or designee, shall establish, document, execute, and maintain
			 * software processes per the requirements in this directive.
			 */
			:>> id = "SWE-005";
			:>> section = "2.1.5.3";
		}

		requirement def <'SWE-140'> SWE140 :> SWERequirement {
			doc /* Center Director, or designee, shall comply with the requirements in this
			 * directive that are marked with an 'X' in Appendix C.
			 */
			:>> id = "SWE-140";
			:>> section = "2.1.5.4";
		}

		requirement def <'SWE-095'> SWE095 :> SWERequirement {
			doc /* The Center Director, or designee, shall report on the status of the Center's
			 * software engineering discipline, as applied to its projects, upon request by the OCE,
			 * OSMA, or OCHMO.
			 */
			:>> id = "SWE-095";
			:>> section = "2.1.5.5";
		}

		requirement def <'SWE-006'> SWE006 :> SWERequirement {
			doc /* Center Director, or designee, shall maintain a reliable list of their Center's
			 * programs and projects containing Class A, B, C, and D software. The list should
			 * include:
			 * a. Project/program name and Work Breakdown Structure (WBS) number.
			 * b. Software name(s) and WBS number(s).
			 * c. Software size estimate (report in Kilo/Thousand Source Lines of Code (KSLOCs)).
			 * d. The phase of development or operations.
			 * e. Software Class or list of the software classes being used on the project.
			 * f. Software safety-critical status.
			 * g. For each Computer Software Configuration Item (CSCI)/Major System containing Class
			 * A, B, or C software, provide:
			 */
			:>> id = "SWE-006";
			:>> section = "2.1.5.6";
		}

		requirement def <'SWE-091'> SWE091 :> SWERequirement {
			doc /* For Class A, B, and C software projects, the Center Director, or designee, shall
			 * establish and maintain a software measurement repository for software project
			 * measurements containing at a minimum:
			 * a. Software development tracking data.
			 * b. Software functionality achieved data.
			 * c. Software quality data.
			 * d. Software development effort and cost data.
			 */
			:>> id = "SWE-091";
			:>> section = "2.1.5.7";
		}

		requirement def <'SWE-092'> SWE092 :> SWERequirement {
			doc /* For Class A, B, and C software projects, the Center Director, or designee, shall
			 * utilize software measurement data for monitoring software engineering capability,
			 * improving software quality, and to track the status of software engineering
			 * improvement activities.
			 */
			:>> id = "SWE-092";
			:>> section = "2.1.5.8";
		}

		requirement def <'SWE-142'> SWE142 :> SWERequirement {
			doc /* For Class A, B, and C software projects, each Center Director, or designee, shall
			 * establish and maintain software cost repository(ies) that contains at least the
			 * following measures:
			 * a. Planned and actual effort and cost.
			 * b. Planned and actual schedule dates for major milestones.
			 * c. Both planned and actual values for key cost parameters that typically include
			 * software size, requirements count, defects counts for maintenance or sustaining
			 * engineering projects, and cost model inputs.
			 * d. Project descriptors or metadata that typically includes software class, software
			 * domain/type, and requirements volatility.
			 */
			:>> id = "SWE-142";
			:>> section = "2.1.5.10";
		}

		requirement def <'SWE-144'> SWE144 :> SWERequirement {
			doc /* Each Center Director, or designee, shall contribute applicable software
			 * engineering process assets in use at his/her Centers to the Agency-wide process asset
			 * library.
			 */
			:>> id = "SWE-144";
			:>> section = "2.1.5.11";
		}

		requirement def <'SWE-153'> SWE153 :> SWERequirement {
			doc /* The designated ETA(s) shall define the content requirements for software
			 * documents or records. .
			 */
			:>> id = "SWE-153";
			:>> section = "2.1.5.12";
		}

		requirement def <'SWE-215'> SWE215 :> SWERequirement {
			doc /* The Center Director, or designee, shall ensure that the Government has clear
			 * rights in the software, a Government purpose license, or other appropriate license or
			 * permission from third party owners prior to providing the software for internal NASA
			 * software sharing or reuse.
			 */
			:>> id = "SWE-215";
			:>> section = "2.1.5.13";
		}

		requirement def <'SWE-216'> SWE216 :> SWERequirement {
			doc /* The Center Director, or designee, shall ensure that all software listed on the
			 * internal software sharing or reuse catalog(s) conforms to NASA software engineering
			 * policy and requirements.
			 */
			:>> id = "SWE-216";
			:>> section = "2.1.5.14";
		}

		requirement def <'SWE-217'> SWE217 :> SWERequirement {
			doc /* The Center Director, or designee, (e.g., the Civil Servant Technical Point of
			 * Contact (POC) for the software product) shall perform the following actions:
			 * a. Keep a list of all contributors to the software product.
			 * b. Ensure that the software product contains appropriate disclaimer and
			 * indemnification provisions (e.g., in a 'README' file) stating that the software may
			 * be subject to U.S. export control restrictions, and it is provided 'as is' without
			 * any warranty, express or implied, and that the recipient waives any claims against,
			 * and indemnifies and holds harmless, NASA and its contractors and subcontractors.
			 */
			:>> id = "SWE-217";
			:>> section = "2.1.5.15";
		}

		requirement def <'SWE-214'> SWE214 :> SWERequirement {
			doc /* The Center Director or designee (e.g., the Civil Servant Technical POC for the
			 * software product) shall perform the following actions for each type of internal NASA
			 * software transfer or reuse:
			 * a. A NASA civil servant to a NASA civil servant:
			 */
			:>> id = "SWE-214";
			:>> section = "2.1.5.16";
		}

		requirement def <'SWE-218'> SWE218 :> SWERequirement {
			doc /* Contracting Officers, as defined in FAR 2.101, or Agreement Managers as defined
			 * in NAII 1050.3, NASA Partnership Guide, in conjunction with Program/Project Managers
			 * shall ensure that the appropriate FAR, NFS, and other provisions/clauses based on
			 * this requirements document and NASA-STD-8739.8 are included for all NASA contracts,
			 * Space Act Agreements, cooperative agreements, partnership agreements, grants, or
			 * other agreements pursuant to which software is being acquired, developed, modified,
			 * operated, or managed for NASA.
			 */
			:>> id = "SWE-218";
			:>> section = "2.1.7.1";
		}

		requirement def <'SWE-126'> SWE126 :> SWERequirement {
			doc /* The technical and institutional authorities for requirements in this directive
			 * shall:
			 * a. Assess projects' requirements mapping matrices and tailoring from requirements in
			 * this directive by:
			 */
			:>> id = "SWE-126";
			:>> section = "2.1.8.2";
		}

		requirement def <'SWE-150'> SWE150 :> SWERequirement {
			doc /* The engineering, CIO, and SMA authorities shall review and agree with any
			 * tailored NPR 7150.2 requirements per the requirements mapping matrix authority
			 * column.
			 */
			:>> id = "SWE-150";
			:>> section = "2.2.7";
		}

		requirement def <'SWE-021'> SWE021 :> SWERequirement {
			doc /* If a system or subsystem development evolves to meet a higher or lower software
			 * classification as defined in Appendix D, then the project manager shall update their
			 * plan(s) and initiate modifications to any supplier contracts to fulfill the
			 * applicable requirements per the Requirements Mapping Matrix in Appendix C with
			 * approved tailoring.
			 */
			:>> id = "SWE-021";
			:>> section = "2.2.8";
		}
	}

	package SoftwareAssurance {
		doc /* The 203 software assurance and software safety tasks of NASA-STD-8739.8B Table 1,
		 * 'Software Assurance and Software Safety Requirements Mapping Matrix', one definition
		 * each. Table 1 repeats the section, SWE number and requirement statement of every row of
		 * the NPR 7150.2D Requirements Mapping Matrix and states the tasks software assurance
		 * performs for it; a definition is named by that SWE number and the number of the task in
		 * its row ('SA-033-1' is task 1 for SWE-033). Table 1 splits SWE-065 into the four rows
		 * 065a to 065d, one per item of the requirement, and the names keep the letter.
		 */

		requirement def <'SWE-033-SA-1'> SA033_1 :> SATask {
			doc /* Confirm that the options for software acquisition versus development have been
			 * evaluated.
			 */
			:>> id = "SWE-033-SA-1";
			:>> sweId = "SWE-033";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-033-SA-2'> SA033_2 :> SATask {
			doc /* Confirm the flow down of applicable software engineering, software assurance, and
			 * software safety requirements on all acquisition activities. (NPR 7150.2 and
			 * NASA-STD-8739.8).
			 */
			:>> id = "SWE-033-SA-2";
			:>> sweId = "SWE-033";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-033-SA-3'> SA033_3 :> SATask {
			doc /* Assess any risks with acquisition versus development decision(s).
			 */
			:>> id = "SWE-033-SA-3";
			:>> sweId = "SWE-033";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-013-SA-1'> SA013_1 :> SATask {
			doc /* Confirm that all plans, including security plans, are in place and have expected
			 * content for the life cycle events, with proper tailoring for the classification of
			 * the software.
			 */
			:>> id = "SWE-013-SA-1";
			:>> sweId = "SWE-013";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-013-SA-2'> SA013_2 :> SATask {
			doc /* Develop and maintain a Software Assurance Plan following the content defined in
			 * NASA-HDBK-2203 for a software assurance plan, including software safety.
			 */
			:>> id = "SWE-013-SA-2";
			:>> sweId = "SWE-013";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-024-SA-1'> SA024_1 :> SATask {
			doc /* Assess plans for compliance with NPR 7150.2 requirements, NASA-STD-8739.8,
			 * including changes to commitments.
			 */
			:>> id = "SWE-024-SA-1";
			:>> sweId = "SWE-024";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-024-SA-2'> SA024_2 :> SATask {
			doc /* Confirm that closure of corrective actions associated with the performance of
			 * software activities against the software plans, including closure rationale.
			 */
			:>> id = "SWE-024-SA-2";
			:>> sweId = "SWE-024";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-024-SA-3'> SA024_3 :> SATask {
			doc /* Confirm changes to commitments are recorded and managed.
			 */
			:>> id = "SWE-024-SA-3";
			:>> sweId = "SWE-024";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-034-SA-1'> SA034_1 :> SATask {
			doc /* Confirm software acceptance criteria are defined and assess the criteria based on
			 * guidance in the NASA Software Engineering Handbook, NASA- HDBK-2203.
			 */
			:>> id = "SWE-034-SA-1";
			:>> sweId = "SWE-034";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-036-SA-1'> SA036_1 :> SATask {
			doc /* Confirm the following are approved, implemented, and updated per requirements: a.
			 * Software processes, including software assurance, software safety, and IV&V
			 * processes, b. Software documentation plans, c. List of developed electronic products,
			 * deliverables, and for the project's d. List of tasks required or needed software
			 * development.
			 */
			:>> id = "SWE-036-SA-1";
			:>> sweId = "SWE-036";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-036-SA-2'> SA036_2 :> SATask {
			doc /* Confirm that any required government actions are established and performed upon
			 * receipt of deliverables (e.g., approvals, reviews).
			 */
			:>> id = "SWE-036-SA-2";
			:>> sweId = "SWE-036";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-037-SA-1'> SA037_1 :> SATask {
			doc /* Confirm that milestones for reviewing and auditing software developer progress
			 * are defined and documented.
			 */
			:>> id = "SWE-037-SA-1";
			:>> sweId = "SWE-037";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-037-SA-2'> SA037_2 :> SATask {
			doc /* Participate in project milestones reviews.
			 */
			:>> id = "SWE-037-SA-2";
			:>> sweId = "SWE-037";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-039-SA-1'> SA039_1 :> SATask {
			doc /* Confirm that software developer(s) periodically report status and provide insight
			 * to the project manager.
			 */
			:>> id = "SWE-039-SA-1";
			:>> sweId = "SWE-039";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-039-SA-2'> SA039_2 :> SATask {
			doc /* Monitor product integration.
			 */
			:>> id = "SWE-039-SA-2";
			:>> sweId = "SWE-039";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-039-SA-3'> SA039_3 :> SATask {
			doc /* Analyze the verification activities to ensure adequacy.
			 */
			:>> id = "SWE-039-SA-3";
			:>> sweId = "SWE-039";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-039-SA-4'> SA039_4 :> SATask {
			doc /* Assess trade studies, source data, software reviews, and technical interchange
			 * meetings.
			 */
			:>> id = "SWE-039-SA-4";
			:>> sweId = "SWE-039";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-039-SA-5'> SA039_5 :> SATask {
			doc /* Perform audits on software development processes and practices at least once
			 * every two years.
			 */
			:>> id = "SWE-039-SA-5";
			:>> sweId = "SWE-039";
			:>> taskNumber = 5;
		}

		requirement def <'SWE-039-SA-6'> SA039_6 :> SATask {
			doc /* Develop and provide status reports.
			 */
			:>> id = "SWE-039-SA-6";
			:>> sweId = "SWE-039";
			:>> taskNumber = 6;
		}

		requirement def <'SWE-039-SA-7'> SA039_7 :> SATask {
			doc /* Develop and maintain a list of all software assurance review discrepancies,
			 * risks, issues, findings, and concerns.
			 */
			:>> id = "SWE-039-SA-7";
			:>> sweId = "SWE-039";
			:>> taskNumber = 7;
		}

		requirement def <'SWE-039-SA-8'> SA039_8 :> SATask {
			doc /* Confirm that the project manager provides responses to software assurance and
			 * software safety submitted issues, findings, and risks and that the project manager
			 * tracks software assurance and software safety issues, findings, and risks to closure.
			 */
			:>> id = "SWE-039-SA-8";
			:>> sweId = "SWE-039";
			:>> taskNumber = 8;
		}

		requirement def <'SWE-040-SA-1'> SA040_1 :> SATask {
			doc /* Confirm that software artifacts are available in electronic format to NASA.
			 */
			:>> id = "SWE-040-SA-1";
			:>> sweId = "SWE-040";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-042-SA-1'> SA042_1 :> SATask {
			doc /* Confirm that software developers provide NASA with electronic access to the
			 * source code generated for the project in a modifiable form.
			 */
			:>> id = "SWE-042-SA-1";
			:>> sweId = "SWE-042";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-139-SA-1'> SA139_1 :> SATask {
			doc /* Assess that the project's software requirements, products, procedures, and
			 * processes are compliant with the NPR 7150.2 requirements per the software
			 * classification and safety criticality for software.
			 */
			:>> id = "SWE-139-SA-1";
			:>> sweId = "SWE-139";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-121-SA-1'> SA121_1 :> SATask {
			doc /* Confirm that any requirement tailoring in the Requirements Mapping Matrix has the
			 * required approvals.
			 */
			:>> id = "SWE-121-SA-1";
			:>> sweId = "SWE-121";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-121-SA-2'> SA121_2 :> SATask {
			doc /* Develop a tailoring matrix of software assurance and software safety
			 * requirements.
			 */
			:>> id = "SWE-121-SA-2";
			:>> sweId = "SWE-121";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-125-SA-1'> SA125_1 :> SATask {
			doc /* Confirm that the project maintains a requirements mapping matrix (matrices) for
			 * all requirements in NPR 7150.2.
			 */
			:>> id = "SWE-125-SA-1";
			:>> sweId = "SWE-125";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-125-SA-2'> SA125_2 :> SATask {
			doc /* Maintain the requirements mapping matrix (matrices) for requirements in
			 * NASA-STD-8739.8.
			 */
			:>> id = "SWE-125-SA-2";
			:>> sweId = "SWE-125";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-027-SA-1'> SA027_1 :> SATask {
			doc /* Confirm that the conditions listed in "a" through "f" are complete for any COTS,
			 * GOTS, MOTS, OSS, or reused software that is acquired or used.
			 */
			:>> id = "SWE-027-SA-1";
			:>> sweId = "SWE-027";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-015-SA-1'> SA015_1 :> SATask {
			doc /* Confirm that the required number of software cost estimates are complete and
			 * include software assurance cost estimate(s) for the project, including a cost
			 * estimate associated with handling safety-critical software and safety-critical data.
			 */
			:>> id = "SWE-015-SA-1";
			:>> sweId = "SWE-015";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-151-SA-1'> SA151_1 :> SATask {
			doc /* Assess the project's software cost estimate(s) to determine if the stated
			 * criteria listed in "a" through "f" are satisfied.
			 */
			:>> id = "SWE-151-SA-1";
			:>> sweId = "SWE-151";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-174-SA-1'> SA174_1 :> SATask {
			doc /* Confirm that all the software planning parameters, including size and effort
			 * estimates, milestones, and characteristics, are submitted to a Center repository.
			 */
			:>> id = "SWE-174-SA-1";
			:>> sweId = "SWE-174";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-174-SA-2'> SA174_2 :> SATask {
			doc /* Confirm that all software assurance and software safety software estimates and
			 * planning parameters are submitted to an organizational repository.
			 */
			:>> id = "SWE-174-SA-2";
			:>> sweId = "SWE-174";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-016-SA-1'> SA016_1 :> SATask {
			doc /* Assess that the software schedule satisfies the conditions in the requirement.
			 */
			:>> id = "SWE-016-SA-1";
			:>> sweId = "SWE-016";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-016-SA-2'> SA016_2 :> SATask {
			doc /* Develop a software assurance schedule, including software assurance products,
			 * audits, reporting, and reviews.
			 */
			:>> id = "SWE-016-SA-2";
			:>> sweId = "SWE-016";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-018-SA-1'> SA018_1 :> SATask {
			doc /* Confirm the generation and distribution of periodic reports on software schedule
			 * activities, metrics, and status, including reports of software assurance and software
			 * safety schedule activities, metrics, and status.
			 */
			:>> id = "SWE-018-SA-1";
			:>> sweId = "SWE-018";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-018-SA-2'> SA018_2 :> SATask {
			doc /* Confirm closure of any project software schedule issues.
			 */
			:>> id = "SWE-018-SA-2";
			:>> sweId = "SWE-018";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-046-SA-1'> SA046_1 :> SATask {
			doc /* Confirm the project's schedules, including the software assurance's/software
			 * safety's schedules, are updated.
			 */
			:>> id = "SWE-046-SA-1";
			:>> sweId = "SWE-046";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-017-SA-1'> SA017_1 :> SATask {
			doc /* Confirm that any project-specific software training has been planned, tracked,
			 * and completed for project personnel, including software assurance and software safety
			 * personnel.
			 */
			:>> id = "SWE-017-SA-1";
			:>> sweId = "SWE-017";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-017-SA-2'> SA017_2 :> SATask {
			doc /* Confirm that software assurance and software safety personnel have completed the
			 * appropriate software assurance and/or software safety training to satisfactorily
			 * conduct assurance and safety activities.
			 */
			:>> id = "SWE-017-SA-2";
			:>> sweId = "SWE-017";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-020-SA-1'> SA020_1 :> SATask {
			doc /* Perform a software classification or concur with the engineering software
			 * classification of software per the descriptions in NPR 7150.2.
			 */
			:>> id = "SWE-020-SA-1";
			:>> sweId = "SWE-020";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-176-SA-1'> SA176_1 :> SATask {
			doc /* Confirm that records of the software Requirements Mapping Matrix and each
			 * software classification are maintained and updated for the life of the project.
			 */
			:>> id = "SWE-176-SA-1";
			:>> sweId = "SWE-176";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-022-SA-1'> SA022_1 :> SATask {
			doc /* Perform software assurance, software safety, and IV&V (if required) according to
			 * the software assurance and software safety standard requirements in NASA-STD-8739.8,
			 * Software Assurance and Project's Software Safety Standard, and the software assurance
			 * plan.
			 */
			:>> id = "SWE-022-SA-1";
			:>> sweId = "SWE-022";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-141-SA-1'> SA141_1 :> SATask {
			doc /* Confirm that IV&V requirements (section 4.4) are complete on projects required to
			 * have IV&V.
			 */
			:>> id = "SWE-141-SA-1";
			:>> sweId = "SWE-141";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-131-SA-1'> SA131_1 :> SATask {
			doc /* Confirm that the IV&V Project Execution Plan (IPEP) exists.
			 */
			:>> id = "SWE-131-SA-1";
			:>> sweId = "SWE-131";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-178-SA-1'> SA178_1 :> SATask {
			doc /* Confirm that IV&V has access to the software development artifacts, products,
			 * source code, and data required to perform the IV&V analysis efficiently and
			 * effectively.
			 */
			:>> id = "SWE-178-SA-1";
			:>> sweId = "SWE-178";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-179-SA-1'> SA179_1 :> SATask {
			doc /* Confirm that the project manager responds to IV&V submitted issues, findings, and
			 * risks and that the project manager tracks IV&V issues, findings, and risks to
			 * closure.
			 */
			:>> id = "SWE-179-SA-1";
			:>> sweId = "SWE-179";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-205-SA-1'> SA205_1 :> SATask {
			doc /* Confirm that the hazard reports or safety data packages contain all known
			 * software contributions or events where software, either by its action, inaction, or
			 * incorrect action, leads to a hazard.
			 */
			:>> id = "SWE-205-SA-1";
			:>> sweId = "SWE-205";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-205-SA-2'> SA205_2 :> SATask {
			doc /* Assess that the hazard reports identify the software components associated with
			 * the system hazards per the criteria defined in NASA-STD-8739.8, Appendix A.
			 */
			:>> id = "SWE-205-SA-2";
			:>> sweId = "SWE-205";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-205-SA-3'> SA205_3 :> SATask {
			doc /* Assess that hazard analyses (including hazard reports) identify the software
			 * components associated with the system hazards per the criteria defined in
			 * NASA-STD-8739.8, Appendix A.
			 */
			:>> id = "SWE-205-SA-3";
			:>> sweId = "SWE-205";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-205-SA-4'> SA205_4 :> SATask {
			doc /* Confirm that the traceability between software requirements and hazards with
			 * software contributions exists.
			 */
			:>> id = "SWE-205-SA-4";
			:>> sweId = "SWE-205";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-205-SA-5'> SA205_5 :> SATask {
			doc /* Develop and maintain a software safety analysis throughout the software
			 * development life cycle.
			 */
			:>> id = "SWE-205-SA-5";
			:>> sweId = "SWE-205";
			:>> taskNumber = 5;
		}

		requirement def <'SWE-023-SA-1'> SA023_1 :> SATask {
			doc /* Confirm that the identified safety-critical software components and data have
			 * implemented the safety- critical software assurance requirements listed in this
			 * standard.
			 */
			:>> id = "SWE-023-SA-1";
			:>> sweId = "SWE-023";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-134-SA-1'> SA134_1 :> SATask {
			doc /* Analyze the software requirements and the software design and work with the
			 * project to implement NPR 7150.2 requirement items "a" through "l."
			 */
			:>> id = "SWE-134-SA-1";
			:>> sweId = "SWE-134";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-134-SA-2'> SA134_2 :> SATask {
			doc /* Assess that the source code satisfies the conditions in the NPR 7150.2
			 * requirement "a" through "l" for safety-critical and mission-critical software at each
			 * code inspection, test review, safety review, and project review milestone.
			 */
			:>> id = "SWE-134-SA-2";
			:>> sweId = "SWE-134";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-134-SA-3'> SA134_3 :> SATask {
			doc /* Confirm that the values of the safety-critical loaded data, uplinked data, rules,
			 * and scripts that affect hazardous system behavior have been tested.
			 */
			:>> id = "SWE-134-SA-3";
			:>> sweId = "SWE-134";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-134-SA-4'> SA134_4 :> SATask {
			doc /* Analyze the software design to ensure the following: a. Use of partitioning or
			 * isolation methods in the design and code, b. That the design logically isolates the
			 * safety-critical design elements and data from those that are non- safety-critical.
			 */
			:>> id = "SWE-134-SA-4";
			:>> sweId = "SWE-134";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-134-SA-5'> SA134_5 :> SATask {
			doc /* Participate in software reviews affecting safety- critical software products.
			 */
			:>> id = "SWE-134-SA-5";
			:>> sweId = "SWE-134";
			:>> taskNumber = 5;
		}

		requirement def <'SWE-134-SA-6'> SA134_6 :> SATask {
			doc /* Ensure the SWE-134 implementation supports and is consistent with the system
			 * hazard analysis.
			 */
			:>> id = "SWE-134-SA-6";
			:>> sweId = "SWE-134";
			:>> taskNumber = 6;
		}

		requirement def <'SWE-219-SA-1'> SA219_1 :> SATask {
			doc /* Confirm that 100% code test coverage is addressed for all identified
			 * safety-critical software components or that software developers provide a technically
			 * acceptable rationale or a risk assessment explaining why the test coverage is not
			 * possible or why the risk does not justify the cost of increasing coverage for the
			 * safety-critical code component.
			 */
			:>> id = "SWE-219-SA-1";
			:>> sweId = "SWE-219";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-220-SA-1'> SA220_1 :> SATask {
			doc /* Perform or analyze Cyclomatic Complexity metrics on all identified
			 * safety-critical software components.
			 */
			:>> id = "SWE-220-SA-1";
			:>> sweId = "SWE-220";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-220-SA-2'> SA220_2 :> SATask {
			doc /* Confirm that all identified safety-critical software components have a cyclomatic
			 * complexity value of 15 or lower. If not, assure that software developers provide a
			 * technically acceptable risk assessment, accepted by the proper technical authority,
			 * explaining why the cyclomatic complexity value needs to be higher than 15 and why the
			 * software component cannot be structured to be lower than 15 or why the cost and risk
			 * of reducing the complexity to below 15 are not justified by the risk inherent in
			 * modifying the software component.
			 */
			:>> id = "SWE-220-SA-2";
			:>> sweId = "SWE-220";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-146-SA-1'> SA146_1 :> SATask {
			doc /* Assess that the approach for the auto-generation software source code is defined,
			 * and the approach 'a' 'g.' satisfies at least the conditions through
			 */
			:>> id = "SWE-146-SA-1";
			:>> sweId = "SWE-146";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-206-SA-1'> SA206_1 :> SATask {
			doc /* Confirm that NASA, engineering, project, software assurance, and IV&V have
			 * electronic access to the models, simulations, and associated data used as inputs for
			 * auto-generation of software.
			 */
			:>> id = "SWE-206-SA-1";
			:>> sweId = "SWE-206";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-032-SA-1'> SA032_1 :> SATask {
			doc /* Confirm that Class A and B software acquired, developed, and maintained by NASA
			 * is performed by an organization with a non-expired CMMI-DEV rating, as per the NPR
			 * 7150.2 requirement.
			 */
			:>> id = "SWE-032-SA-1";
			:>> sweId = "SWE-032";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-032-SA-2'> SA032_2 :> SATask {
			doc /* Assess potential process-related issues, findings, or risks identified from the
			 * CMMI assessment findings.
			 */
			:>> id = "SWE-032-SA-2";
			:>> sweId = "SWE-032";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-032-SA-3'> SA032_3 :> SATask {
			doc /* Perform audits on the software development and software assurance processes.
			 */
			:>> id = "SWE-032-SA-3";
			:>> sweId = "SWE-032";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-147-SA-1'> SA147_1 :> SATask {
			doc /* Confirm that the project has considered reusability for its software development
			 * activities.
			 */
			:>> id = "SWE-147-SA-1";
			:>> sweId = "SWE-147";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-148-SA-1'> SA148_1 :> SATask {
			doc /* Confirm that any project software contributed as a reuse candidate has the
			 * identified information in 'a' 'f.' items through
			 */
			:>> id = "SWE-148-SA-1";
			:>> sweId = "SWE-148";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-156-SA-1'> SA156_1 :> SATask {
			doc /* Confirm the project has performed a software cybersecurity assessment on the
			 * software components per the Agency security policies and the project requirements,
			 * including risks posed by the use of COTS, GOTS, MOTS, OSS, or reused software
			 * components.
			 */
			:>> id = "SWE-156-SA-1";
			:>> sweId = "SWE-156";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-154-SA-1'> SA154_1 :> SATask {
			doc /* Confirm that cybersecurity risks, along with their mitigations, are identified
			 * and managed.
			 */
			:>> id = "SWE-154-SA-1";
			:>> sweId = "SWE-154";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-157-SA-1'> SA157_1 :> SATask {
			doc /* For software products with communications capabilities, confirm that the software
			 * requirements, software design documentation, and software implementation address
			 * unauthorized access per the requirements contained in the Space System Protection
			 * Standard, NASA-STD-1006.
			 */
			:>> id = "SWE-157-SA-1";
			:>> sweId = "SWE-157";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-159-SA-1'> SA159_1 :> SATask {
			doc /* Confirm that testing is complete for the cybersecurity mitigation.
			 */
			:>> id = "SWE-159-SA-1";
			:>> sweId = "SWE-159";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-159-SA-2'> SA159_2 :> SATask {
			doc /* Assess the quality of the cybersecurity mitigation implementation testing and the
			 * test results.
			 */
			:>> id = "SWE-159-SA-2";
			:>> sweId = "SWE-159";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-207-SA-1'> SA207_1 :> SATask {
			doc /* Assess that the software coding guidelines (e.g., coding standards) includes
			 * secure coding practices.
			 */
			:>> id = "SWE-207-SA-1";
			:>> sweId = "SWE-207";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-185-SA-1'> SA185_1 :> SATask {
			doc /* Analyze the engineering data or perform independent static code analysis to
			 * verify that the the project's code meets secure coding standard requirements.
			 */
			:>> id = "SWE-185-SA-1";
			:>> sweId = "SWE-185";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-210-SA-1'> SA210_1 :> SATask {
			doc /* Confirm that the software requirements exist for collecting, reporting, and
			 * storing data relating to the detection of adversarial actions.
			 */
			:>> id = "SWE-210-SA-1";
			:>> sweId = "SWE-210";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-052-SA-1'> SA052_1 :> SATask {
			doc /* Confirm that bi-directional traceability has been completed, recorded, and
			 * maintained.
			 */
			:>> id = "SWE-052-SA-1";
			:>> sweId = "SWE-052";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-052-SA-2'> SA052_2 :> SATask {
			doc /* Confirm that the software traceability includes traceability to any hazard that
			 * includes software.
			 */
			:>> id = "SWE-052-SA-2";
			:>> sweId = "SWE-052";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-050-SA-1'> SA050_1 :> SATask {
			doc /* Confirm that all software requirements are established, captured, and documented
			 * as part of the technical specification, including requirements for COTS, GOTS, MOTS,
			 * OSS, or reused software components.
			 */
			:>> id = "SWE-050-SA-1";
			:>> sweId = "SWE-050";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-051-SA-1'> SA051_1 :> SATask {
			doc /* Perform a software assurance analysis on the detailed software requirements to
			 * analyze the software requirement sources and identify any incorrect, missing, or
			 * incomplete requirements.
			 */
			:>> id = "SWE-051-SA-1";
			:>> sweId = "SWE-051";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-184-SA-1'> SA184_1 :> SATask {
			doc /* Analyze and confirm that the software requirements documentation contains the
			 * software related safety constraints, controls, mitigations, and assumptions between
			 * the hardware, operator, and the software.
			 */
			:>> id = "SWE-184-SA-1";
			:>> sweId = "SWE-184";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-053-SA-1'> SA053_1 :> SATask {
			doc /* Confirm the software requirements changes are documented, tracked, approved, and
			 * maintained throughout the project life cycle.
			 */
			:>> id = "SWE-053-SA-1";
			:>> sweId = "SWE-053";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-054-SA-1'> SA054_1 :> SATask {
			doc /* Monitor identified differences among requirements, project plans, and software
			 * products and confirm differences are addressed and corrective actions are tracked
			 * until closure.
			 */
			:>> id = "SWE-054-SA-1";
			:>> sweId = "SWE-054";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-055-SA-1'> SA055_1 :> SATask {
			doc /* Confirm that the project software testing has shown that software will function
			 * as expected in the customer environment.
			 */
			:>> id = "SWE-055-SA-1";
			:>> sweId = "SWE-055";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-057-SA-1'> SA057_1 :> SATask {
			doc /* Assess that the software architecture addresses or contains the software
			 * structure, qualities, interfaces, and external/internal components.
			 */
			:>> id = "SWE-057-SA-1";
			:>> sweId = "SWE-057";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-057-SA-2'> SA057_2 :> SATask {
			doc /* Analyze the software architecture to assess whether software safety and mission
			 * assurance requirements are met.
			 */
			:>> id = "SWE-057-SA-2";
			:>> sweId = "SWE-057";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-143-SA-1'> SA143_1 :> SATask {
			doc /* Assess the results of or participate in software architecture review activities
			 * held by the project.
			 */
			:>> id = "SWE-143-SA-1";
			:>> sweId = "SWE-143";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-058-SA-1'> SA058_1 :> SATask {
			doc /* Assess the software design against the hardware and software requirements and
			 * identify any gaps.
			 */
			:>> id = "SWE-058-SA-1";
			:>> sweId = "SWE-058";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-058-SA-2'> SA058_2 :> SATask {
			doc /* Assess the software design to verify that the design is consistent with the
			 * software architectural design concepts and that the software design describes the
			 * lower-level units to be coded, compiled, and tested.
			 */
			:>> id = "SWE-058-SA-2";
			:>> sweId = "SWE-058";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-058-SA-3'> SA058_3 :> SATask {
			doc /* Assess that the design does not introduce undesirable behaviors or unnecessary
			 * capabilities.
			 */
			:>> id = "SWE-058-SA-3";
			:>> sweId = "SWE-058";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-058-SA-4'> SA058_4 :> SATask {
			doc /* Confirm that the software design implements all of the required safety-critical
			 * functions and requirements.
			 */
			:>> id = "SWE-058-SA-4";
			:>> sweId = "SWE-058";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-058-SA-5'> SA058_5 :> SATask {
			doc /* Perform a software assurance design analysis.
			 */
			:>> id = "SWE-058-SA-5";
			:>> sweId = "SWE-058";
			:>> taskNumber = 5;
		}

		requirement def <'SWE-060-SA-1'> SA060_1 :> SATask {
			doc /* Confirm that the software code implements the software designs.
			 */
			:>> id = "SWE-060-SA-1";
			:>> sweId = "SWE-060";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-060-SA-2'> SA060_2 :> SATask {
			doc /* Confirm that the code does not contain functionality not defined in the design or
			 * requirements.
			 */
			:>> id = "SWE-060-SA-2";
			:>> sweId = "SWE-060";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-061-SA-1'> SA061_1 :> SATask {
			doc /* Assure the project manager selected and/or defined software coding methods,
			 * standards, and criteria.
			 */
			:>> id = "SWE-061-SA-1";
			:>> sweId = "SWE-061";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-061-SA-2'> SA061_2 :> SATask {
			doc /* Analyze that the software code conforms to all required software coding methods,
			 * rules, and principles.
			 */
			:>> id = "SWE-061-SA-2";
			:>> sweId = "SWE-061";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-135-SA-1'> SA135_1 :> SATask {
			doc /* Analyze the engineering data or perform independent static code analysis to check
			 * for code detects defects, software quality objectives, code coverage objectives,
			 * software complexity values, and software security objectives.
			 */
			:>> id = "SWE-135-SA-1";
			:>> sweId = "SWE-135";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-135-SA-2'> SA135_2 :> SATask {
			doc /* Confirm the static analysis tool(s) are used with checkers to identify security
			 * and coding errors and defects.
			 */
			:>> id = "SWE-135-SA-2";
			:>> sweId = "SWE-135";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-135-SA-3'> SA135_3 :> SATask {
			doc /* Assess that the project addresses the results from the static analysis tools used
			 * by software assurance, software safety, engineering, or the project.
			 */
			:>> id = "SWE-135-SA-3";
			:>> sweId = "SWE-135";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-135-SA-4'> SA135_4 :> SATask {
			doc /* Confirm that the software code has been scanned for security defects and confirm
			 * the result.
			 */
			:>> id = "SWE-135-SA-4";
			:>> sweId = "SWE-135";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-135-SA-5'> SA135_5 :> SATask {
			doc /* Per SWE-219 for safety-critical software, verify code coverage and approved
			 * waivers.
			 */
			:>> id = "SWE-135-SA-5";
			:>> sweId = "SWE-135";
			:>> taskNumber = 5;
		}

		requirement def <'SWE-135-SA-6'> SA135_6 :> SATask {
			doc /* Per SWE-220 for safety-critical software, verify cyclomatic complexity and
			 * approved waivers.
			 */
			:>> id = "SWE-135-SA-6";
			:>> sweId = "SWE-135";
			:>> taskNumber = 6;
		}

		requirement def <'SWE-135-SA-7'> SA135_7 :> SATask {
			doc /* Confirm that Software Quality Objectives or software quality threshold levels are
			 * defined and set for static code analysis defects, checks, or software security
			 * objectives.
			 */
			:>> id = "SWE-135-SA-7";
			:>> sweId = "SWE-135";
			:>> taskNumber = 7;
		}

		requirement def <'SWE-062-SA-1'> SA062_1 :> SATask {
			doc /* Confirm that the project successfully executes the required unit tests,
			 * particularly those testing safety- critical functions.
			 */
			:>> id = "SWE-062-SA-1";
			:>> sweId = "SWE-062";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-062-SA-2'> SA062_2 :> SATask {
			doc /* Confirm that the project addresses or otherwise tracks to closure errors,
			 * defects, or problem reports found during unit testing.
			 */
			:>> id = "SWE-062-SA-2";
			:>> sweId = "SWE-062";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-186-SA-1'> SA186_1 :> SATask {
			doc /* Confirm that the project maintains the procedures, scripts, results, and data
			 * needed to repeat the unit testing (e.g., as-run scripts, test procedures, results).
			 */
			:>> id = "SWE-186-SA-1";
			:>> sweId = "SWE-186";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-063-SA-1'> SA063_1 :> SATask {
			doc /* Confirm that the project creates a correct software version description for each
			 * software release.
			 */
			:>> id = "SWE-063-SA-1";
			:>> sweId = "SWE-063";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-063-SA-2'> SA063_2 :> SATask {
			doc /* For each software release, confirm that the software has been scanned for
			 * security defects and coding standard compliance and confirm the results.
			 */
			:>> id = "SWE-063-SA-2";
			:>> sweId = "SWE-063";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-136-SA-1'> SA136_1 :> SATask {
			doc /* Confirm that the software tool(s) needed to create and maintain software is
			 * validated and accredited.
			 */
			:>> id = "SWE-136-SA-1";
			:>> sweId = "SWE-136";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-065a-SA-1'> SA065a_1 :> SATask {
			doc /* Confirm that software test plans have been established, contain correct content,
			 * and are maintained.
			 */
			:>> id = "SWE-065a-SA-1";
			:>> sweId = "SWE-065";
			:>> sweItem = "a";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-065a-SA-2'> SA065a_2 :> SATask {
			doc /* Confirm that the software test plan addresses the verification of safety-critical
			 * software, specifically the off-nominal scenarios.
			 */
			:>> id = "SWE-065a-SA-2";
			:>> sweId = "SWE-065";
			:>> sweItem = "a";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-065b-SA-1'> SA065b_1 :> SATask {
			doc /* Confirm that the test procedures have been established and are updated when
			 * changes to tests or requirements occur.
			 */
			:>> id = "SWE-065b-SA-1";
			:>> sweId = "SWE-065";
			:>> sweItem = "b";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-065b-SA-2'> SA065b_2 :> SATask {
			doc /* Analyze the software test procedures for the following: a. Coverage of the
			 * software requirements. b. Acceptance or pass/fail criteria, c. The inclusion of
			 * operational and off-nominal conditions, including boundary conditions, d.
			 * Requirements coverage and hazards per SWE-066 and SWE-192, respectively. e.
			 * Requirements coverage for cybersecurity per SWE-157 and SWE-210.
			 */
			:>> id = "SWE-065b-SA-2";
			:>> sweId = "SWE-065";
			:>> sweItem = "b";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-065c-SA-1'> SA065c_1 :> SATask {
			doc /* Confirm that the project creates and maintains any code specifically written to
			 * perform test procedures in a software configuration management system.
			 */
			:>> id = "SWE-065c-SA-1";
			:>> sweId = "SWE-065";
			:>> sweItem = "c";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-065c-SA-2'> SA065c_2 :> SATask {
			doc /* Confirm that the project records all issues and discrepancies in the code
			 * specifically written to perform test procedures.
			 */
			:>> id = "SWE-065c-SA-2";
			:>> sweId = "SWE-065";
			:>> sweItem = "c";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-065c-SA-3'> SA065c_3 :> SATask {
			doc /* Confirm that the project tracks to closure errors and defects found in the code
			 * specifically written to perform test procedures.
			 */
			:>> id = "SWE-065c-SA-3";
			:>> sweId = "SWE-065";
			:>> sweItem = "c";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-065d-SA-1'> SA065d_1 :> SATask {
			doc /* Confirm that the project creates and maintains the test reports throughout
			 * software integration and test.
			 */
			:>> id = "SWE-065d-SA-1";
			:>> sweId = "SWE-065";
			:>> sweItem = "d";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-065d-SA-2'> SA065d_2 :> SATask {
			doc /* Confirm that the project records the test report data and that the data contains
			 * the as-run test data, the test results, and required approvals.
			 */
			:>> id = "SWE-065d-SA-2";
			:>> sweId = "SWE-065";
			:>> sweItem = "d";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-065d-SA-3'> SA065d_3 :> SATask {
			doc /* Confirm that the project records all issues and discrepancies found during each
			 * test.
			 */
			:>> id = "SWE-065d-SA-3";
			:>> sweId = "SWE-065";
			:>> sweItem = "d";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-065d-SA-4'> SA065d_4 :> SATask {
			doc /* Confirm that the project tracks to closure errors and defects found during
			 * testing.
			 */
			:>> id = "SWE-065d-SA-4";
			:>> sweId = "SWE-065";
			:>> sweItem = "d";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-066-SA-1'> SA066_1 :> SATask {
			doc /* Confirm test coverage of the requirements through the execution of the test
			 * procedures.
			 */
			:>> id = "SWE-066-SA-1";
			:>> sweId = "SWE-066";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-066-SA-2'> SA066_2 :> SATask {
			doc /* Perform test witnessing for safety-critical software.
			 */
			:>> id = "SWE-066-SA-2";
			:>> sweId = "SWE-066";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-066-SA-3'> SA066_3 :> SATask {
			doc /* Confirm that any newly identified software contributions to hazards, events, or
			 * conditions found during testing are in the system safety data package.
			 */
			:>> id = "SWE-066-SA-3";
			:>> sweId = "SWE-066";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-187-SA-1'> SA187_1 :> SATask {
			doc /* Confirm that software items to be tested are under configuration management
			 * before the start of testing.
			 */
			:>> id = "SWE-187-SA-1";
			:>> sweId = "SWE-187";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-187-SA-2'> SA187_2 :> SATask {
			doc /* Confirm the project maintains the software items under configuration management
			 * through the completion of testing.
			 */
			:>> id = "SWE-187-SA-2";
			:>> sweId = "SWE-187";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-068-SA-1'> SA068_1 :> SATask {
			doc /* Confirm that test results are assessed and recorded.
			 */
			:>> id = "SWE-068-SA-1";
			:>> sweId = "SWE-068";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-068-SA-2'> SA068_2 :> SATask {
			doc /* Confirm that the project documents software non- conformances in a tracking
			 * system.
			 */
			:>> id = "SWE-068-SA-2";
			:>> sweId = "SWE-068";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-068-SA-3'> SA068_3 :> SATask {
			doc /* Confirm that test results are sufficient verification artifacts for the hazard
			 * reports.
			 */
			:>> id = "SWE-068-SA-3";
			:>> sweId = "SWE-068";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-070-SA-1'> SA070_1 :> SATask {
			doc /* Confirm that the software models, simulations, and analysis tools used to achieve
			 * the qualification of flight software or flight equipment have been validated and
			 * accredited.
			 */
			:>> id = "SWE-070-SA-1";
			:>> sweId = "SWE-070";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-071-SA-1'> SA071_1 :> SATask {
			doc /* Analyze that software test plans and software test procedures cover the software
			 * requirements and provide adequate verification of hazard controls, specifically the
			 * off-nominal scenarios.
			 */
			:>> id = "SWE-071-SA-1";
			:>> sweId = "SWE-071";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-073-SA-1'> SA073_1 :> SATask {
			doc /* Confirm that the project validates the software components on the targeted
			 * platform or a high- fidelity simulation.
			 */
			:>> id = "SWE-073-SA-1";
			:>> sweId = "SWE-073";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-189-SA-1'> SA189_1 :> SATask {
			doc /* Confirm that code coverage measurements have been selected, performed, tracked,
			 * recorded, and communicated with each release.
			 */
			:>> id = "SWE-189-SA-1";
			:>> sweId = "SWE-189";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-190-SA-1'> SA190_1 :> SATask {
			doc /* Confirm that the project performs code coverage analysis using the results of the
			 * tests or a code coverage tool.
			 */
			:>> id = "SWE-190-SA-1";
			:>> sweId = "SWE-190";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-190-SA-2'> SA190_2 :> SATask {
			doc /* Analyze the code coverage measurements to identify uncovered software code.
			 */
			:>> id = "SWE-190-SA-2";
			:>> sweId = "SWE-190";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-190-SA-3'> SA190_3 :> SATask {
			doc /* Assess any uncovered software code for potential risk, issues, or findings.
			 */
			:>> id = "SWE-190-SA-3";
			:>> sweId = "SWE-190";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-191-SA-1'> SA191_1 :> SATask {
			doc /* Confirm that the project plans regression testing and that the regression testing
			 * is adequate and includes retesting of all safety-critical code components.
			 */
			:>> id = "SWE-191-SA-1";
			:>> sweId = "SWE-191";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-191-SA-2'> SA191_2 :> SATask {
			doc /* Confirm that the project performs the planned regression testing.
			 */
			:>> id = "SWE-191-SA-2";
			:>> sweId = "SWE-191";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-191-SA-3'> SA191_3 :> SATask {
			doc /* Identify any risks and issues associated with the regression test set selection
			 * and execution.
			 */
			:>> id = "SWE-191-SA-3";
			:>> sweId = "SWE-191";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-191-SA-4'> SA191_4 :> SATask {
			doc /* Confirm that the regression test procedures are updated to incorporate tests that
			 * validate the correction of critical anomalies.
			 */
			:>> id = "SWE-191-SA-4";
			:>> sweId = "SWE-191";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-192-SA-1'> SA192_1 :> SATask {
			doc /* Through testing, confirm that the project verifies the software requirements
			 * which trace to a hazardous event, cause, or mitigation techniques.
			 */
			:>> id = "SWE-192-SA-1";
			:>> sweId = "SWE-192";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-193-SA-1'> SA193_1 :> SATask {
			doc /* Confirm that the project develops acceptance tests for loaded or uplinked data,
			 * rules, and code that affect software and software system behavior.
			 */
			:>> id = "SWE-193-SA-1";
			:>> sweId = "SWE-193";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-193-SA-2'> SA193_2 :> SATask {
			doc /* Confirm that the loaded or uplinked data, rules, scripts, or code that affect
			 * software and software system behavior are baselined in the software configuration
			 * system.
			 */
			:>> id = "SWE-193-SA-2";
			:>> sweId = "SWE-193";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-193-SA-3'> SA193_3 :> SATask {
			doc /* Confirm that loaded or uplinked data, rules, and scripts are verified as correct
			 * prior to operations, particularly for safety-critical operations.
			 */
			:>> id = "SWE-193-SA-3";
			:>> sweId = "SWE-193";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-211-SA-1'> SA211_1 :> SATask {
			doc /* Confirm that the project is testing COTS, GOTS, MOTS, OSS, or reused software
			 * components to the same level as developed software for its intended use.
			 */
			:>> id = "SWE-211-SA-1";
			:>> sweId = "SWE-211";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-075-SA-1'> SA075_1 :> SATask {
			doc /* Assess the maintenance, operations, and retirement plans for completeness of the
			 * required software engineering and software assurance activities.
			 */
			:>> id = "SWE-075-SA-1";
			:>> sweId = "SWE-075";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-075-SA-2'> SA075_2 :> SATask {
			doc /* Confirm that the project implements software operations, software maintenance,
			 * and software retirement plans.
			 */
			:>> id = "SWE-075-SA-2";
			:>> sweId = "SWE-075";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-077-SA-1'> SA077_1 :> SATask {
			doc /* Confirm that the correct version of the products is delivered, including as-built
			 * documentation and project records.
			 */
			:>> id = "SWE-077-SA-1";
			:>> sweId = "SWE-077";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-077-SA-2'> SA077_2 :> SATask {
			doc /* Perform audits for all deliveries per the configuration management processes to
			 * verify that all products are being delivered and are the correct versions.
			 */
			:>> id = "SWE-077-SA-2";
			:>> sweId = "SWE-077";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-194-SA-1'> SA194_1 :> SATask {
			doc /* Confirm that the project has identified the software requirements to be met, the
			 * approved changes to be implemented, and defects to be resolved for each delivery.
			 */
			:>> id = "SWE-194-SA-1";
			:>> sweId = "SWE-194";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-194-SA-2'> SA194_2 :> SATask {
			doc /* Confirm that the project has met all software requirements identified for
			 * delivery.
			 */
			:>> id = "SWE-194-SA-2";
			:>> sweId = "SWE-194";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-194-SA-3'> SA194_3 :> SATask {
			doc /* Confirm requirements once planned for delivery but no longer appearing in
			 * delivery documentation have been dispositioned.
			 */
			:>> id = "SWE-194-SA-3";
			:>> sweId = "SWE-194";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-194-SA-4'> SA194_4 :> SATask {
			doc /* Confirm that approved changes have been implemented and tested.
			 */
			:>> id = "SWE-194-SA-4";
			:>> sweId = "SWE-194";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-194-SA-5'> SA194_5 :> SATask {
			doc /* Confirm that the approved changes to be implemented and the defects to be
			 * resolved have been resolved.
			 */
			:>> id = "SWE-194-SA-5";
			:>> sweId = "SWE-194";
			:>> taskNumber = 5;
		}

		requirement def <'SWE-194-SA-6'> SA194_6 :> SATask {
			doc /* Approve or sign off on the projects delivered products.
			 */
			:>> id = "SWE-194-SA-6";
			:>> sweId = "SWE-194";
			:>> taskNumber = 6;
		}

		requirement def <'SWE-195-SA-1'> SA195_1 :> SATask {
			doc /* Perform audits on the standards and processes used throughout maintenance based
			 * on the software classification.
			 */
			:>> id = "SWE-195-SA-1";
			:>> sweId = "SWE-195";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-196-SA-1'> SA196_1 :> SATask {
			doc /* Confirm that the project has identified the records and software tools for
			 * archival.
			 */
			:>> id = "SWE-196-SA-1";
			:>> sweId = "SWE-196";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-196-SA-2'> SA196_2 :> SATask {
			doc /* Confirm that the project archives all software and records selected for archival,
			 * as planned.
			 */
			:>> id = "SWE-196-SA-2";
			:>> sweId = "SWE-196";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-079-SA-1'> SA079_1 :> SATask {
			doc /* Assess that a software configuration management plan has been developed and
			 * complies with the requirements in NPR 7150.2 and Center/project guidance.
			 */
			:>> id = "SWE-079-SA-1";
			:>> sweId = "SWE-079";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-080-SA-1'> SA080_1 :> SATask {
			doc /* Analyze proposed software and hardware changes to software products for impacts,
			 * particularly safety and security.
			 */
			:>> id = "SWE-080-SA-1";
			:>> sweId = "SWE-080";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-080-SA-2'> SA080_2 :> SATask {
			doc /* Confirm the following: a. The project tracks the changes. b. The changes are
			 * approved and documented before implementation. c. The implementation of changes is
			 * complete. d. The project tests the changes.
			 */
			:>> id = "SWE-080-SA-2";
			:>> sweId = "SWE-080";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-080-SA-3'> SA080_3 :> SATask {
			doc /* Confirm software changes follow the software change control process.
			 */
			:>> id = "SWE-080-SA-3";
			:>> sweId = "SWE-080";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-081-SA-1'> SA081_1 :> SATask {
			doc /* Confirm that the project has identified the configuration items and their
			 * versions to be controlled.
			 */
			:>> id = "SWE-081-SA-1";
			:>> sweId = "SWE-081";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-081-SA-2'> SA081_2 :> SATask {
			doc /* Assess that the software safety-critical items are configuration-managed,
			 * including hazard reports and safety analysis.
			 */
			:>> id = "SWE-081-SA-2";
			:>> sweId = "SWE-081";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-082-SA-1'> SA082_1 :> SATask {
			doc /* Confirm that software assurance has participation in software control activities.
			 */
			:>> id = "SWE-082-SA-1";
			:>> sweId = "SWE-082";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-082-SA-2'> SA082_2 :> SATask {
			doc /* Perform an audit against the configuration management procedures to confirm that
			 * the project follows the established procedures.
			 */
			:>> id = "SWE-082-SA-2";
			:>> sweId = "SWE-082";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-083-SA-1'> SA083_1 :> SATask {
			doc /* Confirm that the project maintains records of the configuration status of the
			 * configuration items.
			 */
			:>> id = "SWE-083-SA-1";
			:>> sweId = "SWE-083";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-084-SA-1'> SA084_1 :> SATask {
			doc /* Confirm that the project manager performed software configuration audits to
			 * determine the correct version of the software configuration items and verify that the
			 * results of the audit conform to the records that define them.
			 */
			:>> id = "SWE-084-SA-1";
			:>> sweId = "SWE-084";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-085-SA-1'> SA085_1 :> SATask {
			doc /* Confirm that the project establishes procedures for storage, processing,
			 * distribution, release, and support of deliverable software products.
			 */
			:>> id = "SWE-085-SA-1";
			:>> sweId = "SWE-085";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-085-SA-2'> SA085_2 :> SATask {
			doc /* Perform audits on the project to ensure that the project follows defined
			 * procedures for deliverable software products.
			 */
			:>> id = "SWE-085-SA-2";
			:>> sweId = "SWE-085";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-045-SA-1'> SA045_1 :> SATask {
			doc /* Participate in or assess the results from any joint NASA/developer audits. Track
			 * any findings to closure.
			 */
			:>> id = "SWE-045-SA-1";
			:>> sweId = "SWE-045";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-086-SA-1'> SA086_1 :> SATask {
			doc /* Confirm and assess that a risk management process includes recording, analyzing,
			 * planning, tracking, controlling, and communicating all software risks and mitigation
			 * plans.
			 */
			:>> id = "SWE-086-SA-1";
			:>> sweId = "SWE-086";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-086-SA-2'> SA086_2 :> SATask {
			doc /* Perform audits on the risk management process for the software activities.
			 */
			:>> id = "SWE-086-SA-2";
			:>> sweId = "SWE-086";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-087-SA-1'> SA087_1 :> SATask {
			doc /* Confirm that software peer reviews are performed and reported on for project
			 * activities.
			 */
			:>> id = "SWE-087-SA-1";
			:>> sweId = "SWE-087";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-087-SA-2'> SA087_2 :> SATask {
			doc /* Confirm that the project addresses the accepted software peer review findings.
			 */
			:>> id = "SWE-087-SA-2";
			:>> sweId = "SWE-087";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-087-SA-3'> SA087_3 :> SATask {
			doc /* Perform peer reviews on software assurance and software safety plans.
			 */
			:>> id = "SWE-087-SA-3";
			:>> sweId = "SWE-087";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-087-SA-4'> SA087_4 :> SATask {
			doc /* Confirm that the source code satisfies the conditions in the NPR 7150.2
			 * requirement SWE-134, "a" through "l," based upon the software functionality for the
			 * applicable safety-critical requirements at each code inspection/review.
			 */
			:>> id = "SWE-087-SA-4";
			:>> sweId = "SWE-087";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-088-SA-1'> SA088_1 :> SATask {
			doc /* Confirm that the project meets the NPR 7150.2 criteria in "a" through "d" for
			 * each software peer review.
			 */
			:>> id = "SWE-088-SA-1";
			:>> sweId = "SWE-088";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-088-SA-2'> SA088_2 :> SATask {
			doc /* Confirm that the project resolves the actions identified from the software peer
			 * reviews.
			 */
			:>> id = "SWE-088-SA-2";
			:>> sweId = "SWE-088";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-088-SA-3'> SA088_3 :> SATask {
			doc /* Perform audits on the peer-review process.
			 */
			:>> id = "SWE-088-SA-3";
			:>> sweId = "SWE-088";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-089-SA-1'> SA089_1 :> SATask {
			doc /* Confirm that the project records the software peer reviews and results of
			 * software inspection measurements.
			 */
			:>> id = "SWE-089-SA-1";
			:>> sweId = "SWE-089";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-090-SA-1'> SA090_1 :> SATask {
			doc /* Confirm that a measurement program establishes, records, maintains, reports, and
			 * uses software assurance, management, and technical measures.
			 */
			:>> id = "SWE-090-SA-1";
			:>> sweId = "SWE-090";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-090-SA-2'> SA090_2 :> SATask {
			doc /* Perform trending analyses on metrics (quality metrics, defect metrics) and
			 * report.
			 */
			:>> id = "SWE-090-SA-2";
			:>> sweId = "SWE-090";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-090-SA-3'> SA090_3 :> SATask {
			doc /* Collect any identified organizational metrics and submit them to the
			 * organizational repository.
			 */
			:>> id = "SWE-090-SA-3";
			:>> sweId = "SWE-090";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-093-SA-1'> SA093_1 :> SATask {
			doc /* Confirm software measurement data analysis conforms to documented analysis
			 * procedures.
			 */
			:>> id = "SWE-093-SA-1";
			:>> sweId = "SWE-093";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-093-SA-2'> SA093_2 :> SATask {
			doc /* Analyze software assurance measurement data.
			 */
			:>> id = "SWE-093-SA-2";
			:>> sweId = "SWE-093";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-094-SA-1'> SA094_1 :> SATask {
			doc /* Confirm access to software measurement data, analysis, and status as requested to
			 * the following entities, at a minimum: - Sponsoring Mission Directorate - NASA Chief
			 * Engineer - Center Technical Authorities - Headquarters SMA
			 */
			:>> id = "SWE-094-SA-1";
			:>> sweId = "SWE-094";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-199-SA-1'> SA199_1 :> SATask {
			doc /* Confirm that the project monitors and updates planned measurements to ensure the
			 * software meets or exceeds performance and functionality requirements, including
			 * satisfying constraints.
			 */
			:>> id = "SWE-199-SA-1";
			:>> sweId = "SWE-199";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-199-SA-2'> SA199_2 :> SATask {
			doc /* Monitor and track any performance or functionality requirements that are not
			 * being met or are at risk of not being met.
			 */
			:>> id = "SWE-199-SA-2";
			:>> sweId = "SWE-199";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-200-SA-1'> SA200_1 :> SATask {
			doc /* Confirm that the project collects, tracks, and reports on the software volatility
			 * metrics.
			 */
			:>> id = "SWE-200-SA-1";
			:>> sweId = "SWE-200";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-200-SA-2'> SA200_2 :> SATask {
			doc /* Analyze software volatility metrics to evaluate requirements stability as an
			 * early indicator of project problems.
			 */
			:>> id = "SWE-200-SA-2";
			:>> sweId = "SWE-200";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-201-SA-1'> SA201_1 :> SATask {
			doc /* Confirm that all software non-conformances are recorded and tracked to
			 * resolution.
			 */
			:>> id = "SWE-201-SA-1";
			:>> sweId = "SWE-201";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-201-SA-2'> SA201_2 :> SATask {
			doc /* Confirm that accepted non-conformances include the rationale for the
			 * non-conformance.
			 */
			:>> id = "SWE-201-SA-2";
			:>> sweId = "SWE-201";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-202-SA-1'> SA202_1 :> SATask {
			doc /* Confirm that all software non-conformances severity levels are defined.
			 */
			:>> id = "SWE-202-SA-1";
			:>> sweId = "SWE-202";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-202-SA-2'> SA202_2 :> SATask {
			doc /* Assess the application and accuracy of the defined severity levels to software
			 * non-conformances.
			 */
			:>> id = "SWE-202-SA-2";
			:>> sweId = "SWE-202";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-202-SA-3'> SA202_3 :> SATask {
			doc /* Confirm that the project assigns severity levels to non-conformances associated
			 * with tools, COTS, GOTS, MOTS, OSS, and reused software components.
			 */
			:>> id = "SWE-202-SA-3";
			:>> sweId = "SWE-202";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-202-SA-4'> SA202_4 :> SATask {
			doc /* Maintain or access the number of software non- conformances at each severity
			 * level for each software configuration item.
			 */
			:>> id = "SWE-202-SA-4";
			:>> sweId = "SWE-202";
			:>> taskNumber = 4;
		}

		requirement def <'SWE-203-SA-1'> SA203_1 :> SATask {
			doc /* Confirm the evaluations of reported non- conformances for all COTS, GOTS, MOTS,
			 * OSS, or reused software components are occurring throughout the project life cycle.
			 */
			:>> id = "SWE-203-SA-1";
			:>> sweId = "SWE-203";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-203-SA-2'> SA203_2 :> SATask {
			doc /* Assess the impact of non-conformances on the project software's safety, quality,
			 * and reliability.
			 */
			:>> id = "SWE-203-SA-2";
			:>> sweId = "SWE-203";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-204-SA-1'> SA204_1 :> SATask {
			doc /* Perform or confirm that a root cause analysis has been completed on all
			 * identified high severity software nonconformances, and that the results are recorded
			 * and have been assessed for adequacy.
			 */
			:>> id = "SWE-204-SA-1";
			:>> sweId = "SWE-204";
			:>> taskNumber = 1;
		}

		requirement def <'SWE-204-SA-2'> SA204_2 :> SATask {
			doc /* Confirm that the project analyzed the processes identified in the root cause
			 * analysis associated with the high severity software non-conformances.
			 */
			:>> id = "SWE-204-SA-2";
			:>> sweId = "SWE-204";
			:>> taskNumber = 2;
		}

		requirement def <'SWE-204-SA-3'> SA204_3 :> SATask {
			doc /* Assess opportunities for improvement on the processes identified in the root
			 * cause analysis associated with the high severity software non- conformances.
			 */
			:>> id = "SWE-204-SA-3";
			:>> sweId = "SWE-204";
			:>> taskNumber = 3;
		}

		requirement def <'SWE-204-SA-4'> SA204_4 :> SATask {
			doc /* Perform or confirm tracking of corrective actions to closure on high severity
			 * software non- conformances.
			 */
			:>> id = "SWE-204-SA-4";
			:>> sweId = "SWE-204";
			:>> taskNumber = 4;
		}
	}

	/* The project's Requirements Mapping Matrix */

	attribute def MappingCode {
		doc /* The code a MappingKind value writes in the compliance column of a Requirements Mapping
		 * Matrix.
		 */
		attribute code : String;
	}

	enum def MappingKind :> MappingCode {
		doc /* How a project maps to one requirement in its Requirements Mapping Matrix: 'The
		 * Requirements Mapping Matrix documents the program/project's mappings or intent to comply
		 * with the requirements of this NPR or justification for tailoring.' (C.2)
		 * Appendix C prescribes no codes for the column; `code` uses FC, T and NA.
		 */
		fullyCompliant {
			doc /* FC: the project complies with the requirement as written. */
			:>> code = "FC";
		}
		tailored {
			doc /* T: the project tailored the requirement. 'Tailoring. The process used to adjust a
			 * prescribed requirement to accommodate the needs of a specific task or activity
			 * (e.g., program or project). Tailoring may result in changes, subtractions, or
			 * additions to a typical implementation of the requirement.' (Appendix A). 'The
			 * request for relief from a requirement includes the rationale, a risk evaluation, and
			 * reference to all material that justifies supporting acceptance.' (section 2.2.5)
			 */
			:>> code = "T";
		}
		notApplicable {
			doc /* NA: the requirement does not apply to the project. 'Requirements that are not
			 * applicable to a given project, such as the IV&V requirements, should be tailored out
			 * in the Requirements Mapping Matrix with justification.' (C.3)
			 */
			:>> code = "NA";
		}
	}

	metadata def <mapsRequirement> RequirementMapping {
		doc /* One row of a project's Requirements Mapping Matrix: how the project maps to the annotated
		 * SWE requirement or software assurance task usage and, when tailored or not applicable,
		 * the justification Appendix C asks for and the authority that approved it. 'Each project
		 * manager with software components shall maintain a requirements mapping matrix or
		 * multiple requirements mapping matrices against requirements in this NPR, including those
		 * delegated to other parties or accomplished by contract vehicles or Space Act
		 * Agreements. [SWE-125]' `requirement swe141 : SWERequirements::SWE141 {
		 * @RequirementMapping { comply = MappingKind::notApplicable; justification = "..."; } }`
		 */
		:> annotatedElement : SysML::RequirementUsage;
		:> annotatedElement : SysML::RequirementDefinition;
		attribute comply : MappingKind;
		attribute justification : String[0..1];
		attribute approvedBy : String[0..1];
	}

	calc def RequirementMappingEntries :> Query {
		doc /* The requirement usages below root that carry a RequirementMapping annotation. */
		in root : Element;
		in depth : Integer default 16;
		WhereMetadata(
			source = Descendants(source = root, maxDepth = depth),
			'metadata' = "NPR7150::RequirementMapping"
		)
	}

	calc def RequirementsMappingRegister :> Query {
		doc /* A project's Requirements Mapping Matrix generated from its model: one row per mapping
		 * entry of a SWE requirement below root with the Section, SWE #, Requirement Text (the
		 * documentation of the usage's definition), Class A-E Authority and Class F Authority
		 * columns of Table 2 and the project's mapping, justification and approval. The mapping
		 * entries of software assurance tasks are the rows of AssuranceTaskRegister.
		 */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(
				source = RequirementMappingEntries(root = root, depth = depth),
				type = "NPR7150::SWERequirement"
			),
			columns = (
				Column(name = "Section", path = "section"),
				Column(name = "SWE #", path = "id"),
				Column(name = "Requirement Text", path = "type.documentation"),
				Column(name = "Class A-E Authority", path = "authority"),
				Column(name = "Class F Authority", path = "classFAuthority"),
				Column(name = "Comply?", path = "NPR7150::RequirementMapping::comply"),
				Column(name = "Justification", path = "NPR7150::RequirementMapping::justification"),
				Column(name = "Approved By", path = "NPR7150::RequirementMapping::approvedBy")
			)
		)
	}

	calc def AssuranceTaskRegister :> Query {
		doc /* The software assurance and software safety tasks a project mapped, generated from its
		 * model: one row per mapping entry of a NASA-STD-8739.8B Table 1 task below root with the
		 * SWE # of the task's row, the task identifier, the task statement (the documentation of
		 * the usage's definition) and the project's mapping, justification and approval.
		 */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(
				source = RequirementMappingEntries(root = root, depth = depth),
				type = "NPR7150::SATask"
			),
			columns = (
				Column(name = "SWE #", path = "sweId"),
				Column(name = "Task", path = "id"),
				Column(name = "Task Text", path = "type.documentation"),
				Column(name = "Comply?", path = "NPR7150::RequirementMapping::comply"),
				Column(name = "Justification", path = "NPR7150::RequirementMapping::justification"),
				Column(name = "Approved By", path = "NPR7150::RequirementMapping::approvedBy")
			)
		)
	}

	calc def SoftwareClassificationRegister :> Query {
		doc /* One row per system or subsystem containing software below root: name, software class,
		 * safety-critical determination and documentation, the record SWE-176 asks for.
		 */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = depth), type = "NPR7150::SoftwareSystem"),
			properties = ("name", "softwareClass", "safetyCritical", "documentation"),
			tolerate = true
		)
	}

	abstract part def RequirementsMappingMatrix :> Document {
		doc /* The Requirements Mapping Matrix of Appendix C for a software component or system
		 * (SWE-125). Specialize it, state the software class or classes the matrix is for in
		 * `softwareClass`, and add a table whose rows run RequirementsMappingRegister over the
		 * project's root: `part def FlightSoftwareMatrix :> RequirementsMappingMatrix { :>>
		 * softwareClass = SoftwareClass::B; part entries : Section { part matrix : Table { calc
		 * rows : RequirementsMappingRegister { in :>> root = Project; } } } }`. 'Each requirement
		 * marked "X" for the project's software classification(s) should be addressed in the
		 * Requirements Mapping Matrix.' (C.3): a complete matrix has one mapping entry per
		 * requirement SWERequirements marks for one of its classes.
		 */
		attribute softwareClass : SoftwareClass[0..*];
		:>> title default = "NPR 7150.2 Requirements Mapping Matrix";
	}

	view def RequirementsMappingMatrixView :> GridView {
		doc /* Table of the mapping entries, one row per requirement usage that carries a
		 * RequirementMapping annotation.
		 */
		filter @RequirementMapping;
		render asElementTable;
	}

	view def SoftwareClassificationView :> GridView {
		doc /* Table of the systems and subsystems containing software. */
		filter @SoftwareSystemMetadata;
		render asElementTable;
	}

	/* Semantic metadata: the user-defined keywords of the library */

	metadata def <softwareSystem> SoftwareSystemMetadata :> SemanticMetadata {
		doc /* The `#softwareSystem` keyword: classifies a part definition or part usage as a system
		 * or subsystem containing software by applying the definition and the `softwareSystems`
		 * base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = softwareSystems meta SysML::Usage;
	}

	metadata def <sweRequirement> SWERequirementMetadata :> SemanticMetadata {
		doc /* The `#sweRequirement` keyword: classifies a requirement definition or requirement usage
		 * as an SWE requirement by applying the definition and the `sweRequirements` base usage at
		 * once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = sweRequirements meta SysML::Usage;
	}

	metadata def <saTask> SATaskMetadata :> SemanticMetadata {
		doc /* The `#saTask` keyword: classifies a requirement definition or requirement usage as a
		 * software assurance and software safety task by applying the definition and the `saTasks`
		 * base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = saTasks meta SysML::Usage;
	}
}
