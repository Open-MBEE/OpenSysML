library package NASASTD7009 {
	doc /* NON-NORMATIVE OpenSysML vocabulary for NASA Technical Standard NASA-STD-7009B, 'Standard for
	 * Models and Simulations' (Office of the NASA Chief Engineer, approved 2024-03-05, superseding
	 * NASA-STD-7009A w/Change 1), published at
	 * https://standards.nasa.gov/standard/NASA/NASA-STD-7009. The Standard is a work of the United
	 * States Government; the library quotes its text verbatim where the Standard defines a term,
	 * with the section, table, figure or requirement identifier the text comes from. The library
	 * is a community extension: it is not endorsed by NASA and it does not replace the Standard,
	 * NASA-HDBK-7009 or a program's, project's or Center's implementing documents. It lets a
	 * project model, in standard SysML v2, the 43 [M&S n] requirements of section 4 and the
	 * Requirements Identification Matrix of Appendix A, the recommendations of section 4 and the
	 * Recommendations Adoption Matrix of Appendix B, the M&S criticality assessment of Appendix D
	 * (decision consequence, M&S influence and the matrix of Figure 1), the M&S capability and
	 * M&S results assessments of Appendix E with their eleven factors and level definitions, the
	 * content section 4.3.8 asks of a report of M&S results to decision makers, and the M&S life
	 * cycle of Appendix F. Each artefact gets a definition specialising its native SysML type, an
	 * abstract base usage subsetting the native base usage, and a semantic-metadata keyword
	 * applying both at once: `#capabilityAssessment item capability;`, `#resultsReport part
	 * report;`. The Standard 'covers what needs to be accomplished and communicated, not how it
	 * is to be done' (section 1.1), and so does the library: it prescribes no method, threshold
	 * or tool.
	 */

	private import ScalarValues::Integer;
	private import ScalarValues::String;
	private import Metaobjects::SemanticMetadata;
	private import KerML::Root::Element;
	private import Items::Item;
	private import Items::items;
	private import Parts::parts;
	private import Requirements::RequirementCheck;
	private import Requirements::requirementChecks;
	private import StandardViewDefinitions::*;
	private import Views::asElementTable;
	private import DocumentQueries::*;

	/* Appendix F: the M&S life cycle */

	enum def MSLifeCyclePhase {
		doc /* The phases of the M&S life cycle of Appendix F and Figure 6: 'The life cycle of a model
		 * or simulation, like that of any system, has two general parts: M&S development, which
		 * includes M&S initiation, concept development, M&S design, M&S construction, and M&S
		 * testing; and M&S application, which includes use (or operation) and M&S archiving
		 * (including the associated artifacts, products, and analysis performed during a specific
		 * use).' Other life cycles 'are also acceptable if approved by the delegated NASA Technical
		 * Authority'.
		 */
		initiation {
			doc /* M&S initiation: 'the RWS that needs to be modeled and simulated and the information or
			 * results that are required of the M&S are identified and the intended use of the M&S
			 * starts to be defined.'
			 */
		}
		conceptDevelopment {
			doc /* Concept development: 'the aspects of the RWS to be included in the M&S and the
			 * abstractions and assumptions required to implement the M&S are identified. In
			 * addition, the objectives and level of detail required in or from the M&S, and
			 * acceptance criteria to determine M&S sufficiency, are identified.'
			 */
		}
		design {
			doc /* M&S design: 'a conceptual model and other M&S requirements or specifications are
			 * developed that describe the physical behavior and interactions of the RWS from which
			 * a working M&S can be made.' The conceptual model is validated in this phase.
			 */
		}
		construction {
			doc /* M&S construction: 'Once the conceptual model (or M&S design) is validated, the working
			 * M&S is constructed.'
			 */
		}
		testing {
			doc /* M&S testing: verification and empirical validation of the working M&S. 'At the end of
			 * M&S testing, the M&S' capabilities, assumptions, and limits of operation are recorded
			 * and assessed with respect to acceptance criteria [M&S 43] to determine the
			 * permissible uses of the M&S. Once M&S testing is successfully completed, the M&S is
			 * released', ending M&S development.
			 */
		}
		useOrOperation {
			doc /* M&S use (or operation): 'the use of an M&S starts with an assessment of whether or not
			 * the proposed use of the M&S sufficiently matches the permissible use. If the proposed
			 * use is acceptable, the M&S is then used to obtain the results of interest.'
			 */
		}
		archiving {
			doc /* M&S archiving: 'Information regarding the actual use and the specific version of the
			 * M&S used are recorded and archived.'
			 */
		}
	}

	/* Appendix D: the M&S criticality assessment */

	attribute def DecisionConsequenceValue {
		doc /* What a decision consequence level of Table 3 carries: its number, its label, and the
		 * Table 3 cell of each row at that level.
		 */
		attribute level : Integer;
		attribute label : String;
		attribute personnel : String;
		attribute hardwareEquipmentAndFacilities : String {
			doc /* The rows 'Flight or Ground Hardware', 'Flight or Ground Equipment' and 'Flight or
			 * Ground Facilities' of Table 3, which read alike.
			 */
		}
		attribute operationalStatus : String;
		attribute capabilities : String;
		attribute schedules : String;
		attribute cost : String;
		attribute missionSuccessCriteria : String;
	}

	enum def DecisionConsequence :> DecisionConsequenceValue {
		doc /* The decision consequence levels of Appendix D.2 and Table 3: 'Consequence
		 * classifications assess the impact of an M&S-influenced decision that may prove
		 * detrimental to the RWS. The RWS may include any personnel, hardware (including software),
		 * equipment, or facilities acting with or within it.' 'For each row, quantitative
		 * breakpoints should be established between levels 1-5 in coordination with Program/Project
		 * Management and delegated NASA Technical Authority.' The qualifying terms few, many and
		 * most 'are situationally dependent as determined by the affected program, project, or
		 * customer'.
		 */
		negligible {
			doc /* Decision consequence I: Negligible (Table 3). */
			:>> level = 1;
			:>> label = "I: Negligible";
			:>> personnel = "Inconsequential.";
			:>> hardwareEquipmentAndFacilities = "Inconsequential.";
			:>> operationalStatus = "No effect.";
			:>> capabilities = "No effect; no degradation.";
			:>> schedules = "No effect.";
			:>> cost = "No effect.";
			:>> missionSuccessCriteria = "All met.";
		}
		minor {
			doc /* Decision consequence II: Minor (Table 3). */
			:>> level = 2;
			:>> label = "II: Minor";
			:>> personnel = "Minor detriment (first aid).";
			:>> hardwareEquipmentAndFacilities = "Minor detriment (minor maintenance).";
			:>> operationalStatus = "At most a temporary effect.";
			:>> capabilities = "At most a temporary effect; no more than inconsequential degradation.";
			:>> schedules = "Minor impact to schedule with no effect on major mission (operations) milestones.";
			:>> cost = "Minor cost impact but within nominal margins.";
			:>> missionSuccessCriteria = "All met.";
		}
		moderate {
			doc /* Decision consequence III: Moderate (Table 3). */
			:>> level = 3;
			:>> label = "III: Moderate";
			:>> personnel = "Minor injury or occupational illness.";
			:>> hardwareEquipmentAndFacilities = "Minor detriment (maintenance required).";
			:>> operationalStatus = "Temporarily off-line for repair.";
			:>> capabilities = "Temporarily unavailable until restored; some minor degradation.";
			:>> schedules = "Internal schedule slips with no effect on major mission (operations) milestones.";
			:>> cost = "Cost overruns beyond nominal margins, but not detrimental to project execution or full completion.";
			:>> missionSuccessCriteria = "A few not met.";
		}
		significant {
			doc /* Decision consequence IV: Significant (Table 3). */
			:>> level = 4;
			:>> label = "IV: Significant";
			:>> personnel = "Severe injury or occupational illness.";
			:>> hardwareEquipmentAndFacilities = "Major damage.";
			:>> operationalStatus = "Permanently degraded until repaired.";
			:>> capabilities = "Significant or permanent degradation until repaired.";
			:>> schedules = "Impacts to major mission (operations) milestones.";
			:>> cost = "Cost overruns detrimental to program or project execution or completion.";
			:>> missionSuccessCriteria = "Many not met.";
		}
		catastrophic {
			doc /* Decision consequence V: Catastrophic (Table 3). */
			:>> level = 5;
			:>> label = "V: Catastrophic";
			:>> personnel = "Permanent disability or death.";
			:>> hardwareEquipmentAndFacilities = "Irrecoverable total loss of function or destroyed.";
			:>> operationalStatus = "Non-operational.";
			:>> capabilities = "Severely degraded to none.";
			:>> schedules = "Operational (e.g., mission) windows missed.";
			:>> cost = "Cost overruns cause major program or project reductions or cancellation.";
			:>> missionSuccessCriteria = "Most to all not met.";
		}
	}

	attribute def MSInfluenceValue {
		doc /* What an M&S influence level of Appendix D.3 carries: its number, its label, its
		 * description, and its row of Figure 1 as the lowest decision consequence level the row
		 * rates yellow and the lowest it rates red (absent where the row has no red cell).
		 */
		attribute level : Integer;
		attribute label : String;
		attribute description : String;
		attribute yellowFrom : Integer;
		attribute redFrom : Integer[0..1];
	}

	enum def MSInfluence :> MSInfluenceValue {
		doc /* The M&S influence levels of Appendix D.3 and Table 4: 'M&S Influence estimates the
		 * degree to which M&S results impact the (program/project) decision under consideration
		 * (see Table 4, M&S Influence). This is predicated on the amount of other information
		 * available when making the impending decision. (Engineering decisions include
		 * determination of whether design requirements have been verified.)' Each literal's
		 * documentation gives the Table 4 conditions on the information available for the real
		 * system in the real environment, for a similar system in a similar environment, and from
		 * other M&S.
		 */
		negligible {
			doc /* M&S influence 1: Negligible. The decision support information is primarily from
			 * observations of the real system supported by information from a similar system or
			 * information from a similar credible M&S, with minimal support provided by the
			 * current M&S information. Table 4: ample flight or test information for the real
			 * system in the real environment is available, flight or test data for similar
			 * systems in similar environments are immaterial, and no other M&S or analysis
			 * information is available; or some flight or test information for the real system
			 * in the real environment is available, flight or test data for similar systems in
			 * similar environments are available, and credible results from another M&S are
			 * available.
			 */
			:>> level = 1;
			:>> label = "1: Negligible";
			:>> description = "The decision support information is primarily from observations of the real system supported by information from a similar system or information from a similar credible M&S, with minimal support provided by the current M&S information.";
			:>> yellowFrom = 5;
		}
		minor {
			doc /* M&S influence 2: Minor. The decision support information consists of observations
			 * in the real system, empirical information from a similar system, or information
			 * from a similar credible M&S, with limited support by the current M&S information.
			 * Table 4: some flight or test information for the real system in the real
			 * environment is available, it is immaterial if flight or test data for similar
			 * systems in similar environments are available, and no other M&S or analysis data
			 * are available; or limited flight or test information for the real system in the
			 * real environment is available, some flight or test information for similar
			 * systems in similar environments is available, and credible results from another
			 * M&S are available.
			 */
			:>> level = 2;
			:>> label = "2: Minor";
			:>> description = "The decision support information consists of observations in the real system, empirical information from a similar system, or information from a similar credible M&S, with limited support by the current M&S information.";
			:>> yellowFrom = 4;
		}
		moderate {
			doc /* M&S influence 3: Moderate. The decision support information is balanced between
			 * the M&S and some observational information from the real system, empirical
			 * information from a similar system, or information from a similar credible M&S.
			 * Table 4: limited flight or test information for the real system in the real
			 * environment are available, ample flight or test information for similar systems
			 * in similar environments are available, and no other M&S or analysis data are
			 * available; or no information is available for the real system in the real
			 * environment, ample flight or test information for similar systems in similar
			 * environments is available, and credible results from another M&S are available.
			 */
			:>> level = 3;
			:>> label = "3: Moderate";
			:>> description = "The decision support information is balanced between the M&S and some observational information from the real system, empirical information from a similar system, or information from a similar credible M&S.";
			:>> yellowFrom = 2;
			:>> redFrom = 5;
		}
		significant {
			doc /* M&S influence 4: Significant. Decision support information is primarily from the
			 * M&S but has support from another credible model or empirical similar system
			 * information. Table 4: no available information for the real system in the real
			 * environment, ample flight or test data for similar systems in similar
			 * environments are available, and no other M&S or analysis data are available; or
			 * no available information for the real system in the real environment, no
			 * information is available for a similar system in similar environment, and
			 * credible results from another M&S are available.
			 */
			:>> level = 4;
			:>> label = "4: Significant";
			:>> description = "Decision support information is primarily from the M&S but has support from another credible model or empirical similar system information.";
			:>> yellowFrom = 2;
			:>> redFrom = 4;
		}
		controlling {
			doc /* M&S influence 5: Controlling. The only decision support information is the M&S.
			 * Table 4: no available information for the real system in the real environment, no
			 * available information for a similar system in similar environment, and no other
			 * M&S or analysis data are available.
			 */
			:>> level = 5;
			:>> label = "5: Controlling";
			:>> description = "The only decision support information is the M&S.";
			:>> yellowFrom = 2;
			:>> redFrom = 3;
		}
	}

	enum def CriticalityRating {
		doc /* The three cases of the M&S Criticality Assessment Matrix of Figure 1 (Appendix D.1):
		 * 'From the perspective of situational criticality, the three possible cases for assessment
		 * are' the red, yellow and green boxes.
		 */
		green {
			doc /* (G): 'There is not a critical driving force for those falling within the green (G)
			 * boxes.'
			 */
		}
		yellow {
			doc /* (Y): 'The M&S that are assessed to fall within the yellow (Y) boxes are at the
			 * discretion of program/project management in collaboration with the delegated NASA
			 * Technical Authority for adhering to the requirements of this Standard.'
			 */
		}
		red {
			doc /* (R): 'Those M&S that are assessed to fall within the red (R) boxes in Figure 1 are
			 * clear candidates for completely adhering to the requirements of this Standard.'
			 */
		}
	}

	calc def CriticalityOf {
		doc /* The cell of Figure 1, M&S Criticality Assessment Matrix, for a decision consequence and
		 * an M&S influence. Row 5 Controlling reads G Y R R R over consequences I to V, row 4
		 * Significant G Y Y R R, row 3 Moderate G Y Y Y R, row 2 Minor G G G Y Y and row 1
		 * Negligible G G G G Y. 'Programs, projects, stakeholders, or customers of M&S may find
		 * the need to adjust this matrix'; a project that does specialises this calculation and
		 * records the method in its CriticalityAssessment.
		 */
		in consequence : DecisionConsequence;
		in influence : MSInfluence;
		return rating : CriticalityRating =
			if influence.level == 5 ? (
				if consequence.level >= 3 ? CriticalityRating::red
				else if consequence.level == 2 ? CriticalityRating::yellow
				else CriticalityRating::green
			)
			else if influence.level == 4 ? (
				if consequence.level >= 4 ? CriticalityRating::red
				else if consequence.level >= 2 ? CriticalityRating::yellow
				else CriticalityRating::green
			)
			else if influence.level == 3 ? (
				if consequence.level == 5 ? CriticalityRating::red
				else if consequence.level >= 2 ? CriticalityRating::yellow
				else CriticalityRating::green
			)
			else if influence.level == 2 ? (
				if consequence.level >= 4 ? CriticalityRating::yellow
				else CriticalityRating::green
			)
			else (
				if consequence.level == 5 ? CriticalityRating::yellow
				else CriticalityRating::green
			);
	}

	metadata def <criticality> CriticalityAssessment {
		doc /* The record of the assessed criticality associated with M&S use that [M&S 6] asks for,
		 * on the model, simulation, analysis or decision it is assessed for: 'An M&S criticality
		 * assessment considers: (1) the consequences to human safety or mission success criteria,
		 * and (2) the degree to which M&S results influence a decision.' (Appendix D.1). `rating`
		 * is the cell of the matrix used; `method` records the method when it is not Figure 1 as
		 * is: 'The method used, either Appendix D as is or modified, or another method, is to be
		 * recorded.' (section 4.1.1.2). `analysis def Stress { @CriticalityAssessment { consequence
		 * = DecisionConsequence::significant; influence = MSInfluence::controlling; rating =
		 * CriticalityRating::red; } }`
		 */
		attribute consequence : DecisionConsequence;
		attribute influence : MSInfluence;
		attribute rating : CriticalityRating[0..1];
		attribute method : String[0..1];
		attribute basis : String[0..1];
	}

	/* Appendix E: the M&S capability assessment and the M&S results assessment */

	attribute def FactorLevelValue {
		doc /* The ordinal index of an assessment level. */
		attribute level : Integer;
	}

	enum def FactorLevel :> FactorLevelValue {
		doc /* The five assessment levels of Appendix E.2: 'Reporting factor level assessment is
		 * recommended using a 5-tier labeling system representing levels ranging from "no
		 * supporting information" to "gold-standard" evidence and rigor. An ordinal indexing system
		 * of assessment labels (e.g., 0-4) is utilized for illustration'. 'The assessment of each
		 * factor level is a discrete step function, with no intentions for partial credit at any
		 * given level.' 'Given that the factors are largely independent, comparison between or
		 * combining of factor assessments and thresholds is not intended and should be avoided in
		 * all labeling approaches.'
		 */
		level0 { :>> level = 0; }
		level1 { :>> level = 1; }
		level2 { :>> level = 2; }
		level3 { :>> level = 3; }
		level4 { :>> level = 4; }
	}

	attribute def AssessmentFactorValue {
		doc /* What an assessment factor of Appendix E carries: its title, the section and table that
		 * define it, the question its assessment asks (sections E.3.1 and E.4.1), and the evidence
		 * its level definitions ask for at levels 0 to 4.
		 */
		attribute title : String;
		attribute section : String;
		attribute levelTable : String;
		attribute question : String;
		attribute level0 : String;
		attribute level1 : String;
		attribute level2 : String;
		attribute level3 : String;
		attribute level4 : String;
	}

	enum def CapabilityFactor :> AssessmentFactorValue {
		doc /* The five factors of the M&S capability assessment of Appendix E.3 and Table 5, which
		 * [M&S 48] has the assessed M&S capability recorded by: 'For the M&S capability
		 * assessment, a set of five factors are identified, all of which are associated with
		 * processes that take place during the development phase of the M&S life cycle.' 'The
		 * M&S capability assessment addresses only the outcomes of M&S development and occurs
		 * no later than the end of that life-cycle phase, which is marked by the release of the
		 * M&S.' (Appendix E.2)
		 */
		msDataPedigree {
			doc /* M&S Data Pedigree (section E.3.2, Table 7). Data Pedigree 'is formally defined as
			 * a record of the traceability of data from their source through all aspects of
			 * their transmission, storage, and processing to their final form used in the
			 * development of an M&S.'
			 */
			:>> title = "M&S Data Pedigree";
			:>> section = "E.3.2";
			:>> levelTable = "Table 7";
			:>> question = "Is the pedigree (and quality) of the data used to develop the M&S adequate or acceptable?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "Some data are known and traceable to informal documentation. Sources of all significant data are known. Uncertainties in data may not even be estimated.";
			:>> level2 = "Most data are known and traceable to formal documentation. Processes to establish significant data are known. Uncertainties in all data are at least estimated.";
			:>> level3 = "All data are known and can be traced to a sufficiently representative referent. All significant data are acceptable in terms of accuracy, precision and uncertainty.";
			:>> level4 = "All data are known and fully traceable to the RWS. All data are acceptable in terms of accuracy, precision, and uncertainty.";
		}
		msVerification {
			doc /* M&S Verification (section E.3.3, Table 8). 'Verification includes all activities
			 * that are performed to provide assurance that the conceptual and mathematical
			 * models are correct, specifications for the computational model are sufficient and
			 * accurate, and the computational model is correctly implemented.'
			 */
			:>> title = "M&S Verification";
			:>> section = "E.3.3";
			:>> levelTable = "Table 8";
			:>> question = "Were the M&S implemented correctly, per their requirements/specifications?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "Verification is informal, with some documentation or evidence of completeness/success.";
			:>> level2 = "The model is correctly implemented as determined by documented verification practices, which evaluate all components, features, capabilities, and couplings of the model. Documented methods are used to assess model errors. Most of the important model errors satisfy program/project-specified requirements.";
			:>> level3 = "The model is correctly implemented as determined by rigorous verification practices, which evaluate all components, features, capabilities, and couplings of the end-to-end model. Rigorous methods are used to assess model errors. All important model errors satisfy program/project-specified requirements.";
			:>> level4 = "The model is correctly implemented as determined by reliable verification practices, which evaluate all components, features, capabilities, and couplings of the end-to-end model. Reliable estimation methods are used to assess model errors. All model errors satisfy program/project specified requirements.";
		}
		msValidation {
			doc /* M&S Validation (section E.3.4, Table 9). 'For the validation factor, an
			 * assessment of any level above "1" is not permitted unless the conditions for "1"
			 * are satisfied, i.e., the model has to be conceptually validated before it is
			 * empirically validated.'
			 */
			:>> title = "M&S Validation";
			:>> section = "E.3.4";
			:>> levelTable = "Table 9";
			:>> question = "Did the M&S results compare favorably to the referent data, and how close is the referent to the RWS?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "The model is conceptually validated. The problem statement (intended use) is clearly stated and well-understood; and the conceptual model, requirements, and specifications are correct and sufficiently address the problem.";
			:>> level2 = "M&S results compare favorably to measurements from a representative system or to results from a higher-fidelity M&S that satisfies the conditions for Level '2'. Validation points are within the domain of operation for the RWS. Favorable comparisons are obtained for at least some of the important response quantities.";
			:>> level3 = "M&S results compare favorably to measurements on the RWS in a representative environment or to results from a higher-fidelity M&S that satisfies the conditions for Level '3'. Validation points significantly span the domain of operation for the RWS. Favorable comparisons are obtained for all important response quantities.";
			:>> level4 = "M&S results compare favorably to measurements on the RWS in its operating environment or to results from a higher-fidelity M&S that satisfies the conditions for Level '4'. Validation points completely span the domain of operation for the RWS. Favorable comparisons are obtained for all response quantities.";
		}
		msDevelopmentTechnicalReview {
			doc /* M&S Development Technical Review (section E.3.5, Table 10). 'While this Standard
			 * does not require technical reviews in support of, or in association with, the M&S
			 * development, the results of any technical reviews accomplished during the entire
			 * life cycle of the M&S are to be recorded per [M&S 9]; and the findings of these
			 * reviews are to be reported to decision makers per [M&S 36].'
			 */
			:>> title = "M&S Development Technical Review";
			:>> section = "E.3.5";
			:>> levelTable = "Table 10";
			:>> question = "Were any technical reviews of the M&S or M&S development accomplished? What were the scopes of the reviews? How formal were the reviews? How recently did the reviews occur, relative to the development activities? Who conducted the reviews, and how independent were the reviewers from the M&S program/project?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "Favorable informal internal peer review. Scope addresses Data Pedigree, Verification, and Validation factors, at a minimum.";
			:>> level2 = "Favorable formal internal peer review. Scope addresses Data Pedigree, Verification, and Validation factors, at a minimum.";
			:>> level3 = "Favorable independent peer review. Scope addresses all other capability-related factors.";
			:>> level4 = "Favorable independent peer review and independent factor evaluation. Scope addresses all other capability-related factors.";
		}
		msDevelopmentProcessProductManagement {
			doc /* M&S Development Process/Product Management (section E.3.6, Table 11). 'For the
			 * M&S Development Process/Product Management factor, an assessment at a Level of
			 * either "3" or "4" requires the conditions for a Level of "2" be satisfied.'
			 */
			:>> title = "M&S Development Process/Product Management";
			:>> section = "E.3.6";
			:>> levelTable = "Table 11";
			:>> question = "How well managed were the M&S development processes and products?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "Roles and responsibilities are defined in the context of an M&S process that is informally documented. Requirements for M&S products are informally documented. CM of M&S products is established and applied using informal methods.";
			:>> level2 = "Roles and responsibilities are defined in the context of an M&S process that is formally documented and approved. Requirements for M&S products are formally documented and approved. CM of M&S products is established and applied using formal methods.";
			:>> level3 = "The formally established process is rigorously controlled and followed. Compliance with the process is formally documented. Measurements of process and product compliance are made and documented. CM of M&S products is rigorously applied.";
			:>> level4 = "Measurements, including customer/user feedback, are used to improve both the M&S process and products.";
		}
	}

	enum def ResultsFactor :> AssessmentFactorValue {
		doc /* The six factors of the M&S results assessment of Appendix E.4 and Table 12, which
		 * [M&S 31] has the assessed M&S results recorded by: 'For the M&S results assessment, a
		 * set of six factors are identified, all of which are associated with processes that
		 * take place during the use phase of the M&S life cycle.' 'The M&S results assessment
		 * addresses the outcomes of M&S use and occurs at or toward the end of that phase when
		 * the results are obtained.' (Appendix E.2)
		 */
		msUseAssessment {
			doc /* M&S Use Assessment (section E.4.2, Table 15). 'This Standard defines the use
			 * assessment as the process of determining if an M&S is accepted for a proposed
			 * use.' The proposed use [M&S 22] is assessed [M&S 23] against the permissible uses
			 * [M&S 14].
			 */
			:>> title = "M&S Use Assessment";
			:>> section = "E.4.2";
			:>> levelTable = "Table 15";
			:>> question = "How well does the proposed M&S use match the permissible use(s) of the M&S? How similar is the proposed use of the M&S to previous successful uses?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "M&S type, application domain, and purpose match the proposed use. The M&S' inclusions, exclusions, abstractions, and assumptions are acceptable for the proposed use. The M&S is able to produce the desired outputs. The proposed use departs substantially from prior uses of the M&S, or this is the first use of the M&S.";
			:>> level2 = "The proposed use is within the M&S limits. The proposed use differs in moderate ways from prior M&S uses.";
			:>> level3 = "The proposed use is within the domain of M&S verification. The proposed use differs in minor ways from prior M&S uses.";
			:>> level4 = "The proposed use is within the domain of M&S validation. The proposed use is nearly identical to previous M&S uses.";
		}
		msInputPedigree {
			doc /* M&S Input Pedigree (section E.4.3, Table 16). Input pedigree 'is formally defined
			 * as a record of the traceability of data from its source through all aspects of
			 * its transmission, storage, and processing to its final form when using an M&S.'
			 */
			:>> title = "M&S Input Pedigree";
			:>> section = "E.4.3";
			:>> levelTable = "Table 16";
			:>> question = "Is the pedigree (and quality) of the data used to setup and run the model adequate or acceptable?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "Some input data are known and traceable to informal documentation. Sources of all significant input data are known. Uncertainties in input data may not even be estimated.";
			:>> level2 = "Most input data are known and traceable to formal documentation. Processes to establish significant data are known. Uncertainties in all data are at least estimated.";
			:>> level3 = "All input data are known and can be traced to a sufficiently representative referent. All significant data are acceptable in terms of accuracy, precision, and uncertainty.";
			:>> level4 = "All input data known and fully traceable to the RWS. All data are acceptable in terms of accuracy, precision, and uncertainty.";
		}
		msUncertaintyCharacterization {
			doc /* M&S Uncertainty Characterization (section E.4.4, Table 17). 'Uncertainty
			 * characterization includes the identification of uncertainty sources and the
			 * qualification or quantification of uncertainty in the current M&S results.'
			 */
			:>> title = "M&S Uncertainty Characterization";
			:>> section = "E.4.4";
			:>> levelTable = "Table 17";
			:>> question = "Is the uncertainty in the current M&S results appropriately characterized? What are the sources of uncertainty in the results and how are they propagated through to the results of the analysis?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "Sources of input uncertainty are identified with qualitative estimates of the uncertainty. Their impact on output uncertainties and uncertainty propagation are not addressed.";
			:>> level2 = "Most sources of uncertainty are identified, expressed quantitatively, and correctly classified based on SME opinions and/or by deduction from experimental data. Propagation of the uncertainty to output quantities are addressed by reduced order (e.g., monotonic or boundary analysis) and/or reduced dimension propagation.";
			:>> level3 = "Quantitative estimates of uncertainties are reported for most output quantities after a propagation of all known sources of uncertainty.";
			:>> level4 = "A statistical analysis (e.g., ANOVA) of the output uncertainty is performed for all output quantities after rigorous and validated propagation of all known sources of uncertainty. Reported results may include statistical moments, confidence intervals, sensitivity analysis, etc.";
		}
		msResultsRobustness {
			doc /* M&S Results Robustness (section E.4.5, Table 18). 'Results robustness is the
			 * determination of how thoroughly the sensitivities of the current M&S results (to
			 * the variables and parameters of the M&S) are known.' As a guideline, 'few'
			 * implies the sensitivity of less than 20 percent of the potential variables and
			 * parameters is known, 'many' between 20 and 50 percent, and 'most' the majority,
			 * including all of the most sensitive ones.
			 */
			:>> title = "M&S Results Robustness";
			:>> section = "E.4.5";
			:>> levelTable = "Table 18";
			:>> question = "How thoroughly are the sensitivities of the current M&S results known?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "Sensitivity of M&S results for the RWS is estimated by analogy with the quantified sensitivity of similar problems of interest.";
			:>> level2 = "Sensitivity of the M&S results for the RWS is quantitatively known for a few variables and parameters. Only a few (or none) of the most sensitive variables and parameters are identified. Sensitivities of combinations of variables and parameters are not known.";
			:>> level3 = "Sensitivity of the M&S results for the RWS is quantitatively known for many variables and parameters, including many of the most sensitive variables and parameters. Sensitivities of some combinations of these variables and parameters are also quantified.";
			:>> level4 = "Sensitivity of the M&S results for the RWS is quantitatively known for most of the variables and parameters, including most, if not all, of the most sensitive variables and parameters. Sensitivities of many combinations of these variables and parameters are also quantified.";
		}
		msUseAnalysisTechnicalReview {
			doc /* M&S Use/Analysis Technical Review (section E.4.6, Table 19). 'The results of any
			 * technical reviews accomplished during the entire life cycle of the M&S are to be
			 * recorded per [M&S 9], and the findings of these reviews are to be reported to
			 * decision makers per [M&S 36].'
			 */
			:>> title = "M&S Use/Analysis Technical Review";
			:>> section = "E.4.6";
			:>> levelTable = "Table 19";
			:>> question = "Were any technical reviews of the M&S use or M&S results accomplished? What were the scopes of the reviews? How formal were the reviews? How recently did the reviews occur relative to the use and analysis activities? Who conducted the reviews, and how independent were the reviewers from the M&S program/project?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "Favorable informal internal peer review. Scope addresses Input Pedigree, Uncertainty Characterization, and Results Robustness factors, at a minimum.";
			:>> level2 = "Favorable formal internal peer review. Scope addresses Input Pedigree, Uncertainty Characterization, and Results Robustness factors, at a minimum.";
			:>> level3 = "Favorable independent peer review. Scope addresses all other results-related factors.";
			:>> level4 = "Favorable independent peer review and independent factor evaluation. Scope addresses all other capability-related factors.";
		}
		msUseProcessProductManagement {
			doc /* M&S Use Process/Product Management (section E.4.7, Table 20). 'For the M&S use
			 * process/product management factor, an assessment at a level of either "3" or "4"
			 * requires the conditions for a level of "2" be satisfied.'
			 */
			:>> title = "M&S Use Process/Product Management";
			:>> section = "E.4.7";
			:>> levelTable = "Table 20";
			:>> question = "How well managed were the M&S Use processes and products?";
			:>> level0 = "Insufficient evidence.";
			:>> level1 = "Roles and responsibilities are defined in the context of an M&S process that is informally documented. Requirements for M&S products are informally documented. CM of M&S products is established and applied using informal methods.";
			:>> level2 = "Roles and responsibilities are defined in the context of an M&S process that is formally documented and approved. Requirements for M&S products are formally documented and approved. CM of M&S products is established and applied using formal methods.";
			:>> level3 = "The formally established process is rigorously controlled and followed. Compliance with the process is formally documented. Measurements of process and product compliance are made and documented. CM of M&S products is rigorously applied.";
			:>> level4 = "Measurements, including customer/user feedback, are used to improve both the M&S process and products.";
		}
	}
	abstract item def FactorAssessment :> Item {
		doc /* The assessment of one factor: the level assessed, the threshold set for it, and the
		 * evidence the level rests on. 'This Standard levies no requirements with respect to what
		 * levels to achieve (the sufficiency threshold levels), merely that the levels be
		 * determined and reported.' (section 4.3.6). The threshold is the M&S assessment level
		 * threshold of [M&S 43] e; the outcome reported to decision makers 'is understood to
		 * include the assessed levels for each' factor 'as well as the gaps between the assessed
		 * levels and the thresholds as established per [M&S 43e]' (sections 4.3.8.4 and 4.3.8.5).
		 */
		attribute level : FactorLevel;
		attribute threshold : FactorLevel[0..1];
		attribute evidence : String[0..1];
	}

	item def CapabilityFactorAssessment :> FactorAssessment {
		doc /* The assessed level of one M&S capability factor (Appendix E.3). */
		attribute factor : CapabilityFactor;
	}

	abstract item capabilityFactorAssessments : CapabilityFactorAssessment[0..*] nonunique :> items;

	item def ResultsFactorAssessment :> FactorAssessment {
		doc /* The assessed level of one M&S results factor (Appendix E.4). */
		attribute factor : ResultsFactor;
	}

	abstract item resultsFactorAssessments : ResultsFactorAssessment[0..*] nonunique :> items;

	item def MSCapabilityAssessment :> Item {
		doc /* The record of the assessed M&S capability: 'A record of the assessed M&S capability shall
		 * be maintained according to each of the following factors: a. M&S Data Pedigree. b. M&S
		 * Verification. c. M&S Validation. d. M&S Development Technical Review. e. M&S Development
		 * Process/Product Management.' [M&S 48]. 'The capability assessment is a product of the
		 * development phase' and 'is still to be performed at or prior to the release of the M&S'
		 * (Appendix E.2). A project nests one CapabilityFactorAssessment per factor.
		 */
		item factorAssessments : CapabilityFactorAssessment[0..*] :> capabilityFactorAssessments;
	}

	abstract item capabilityAssessments : MSCapabilityAssessment[0..*] nonunique :> items;

	item def MSResultsAssessment :> Item {
		doc /* The record of the assessed M&S results: 'A record of the assessed M&S results shall be
		 * maintained according to each of the following factors: a. M&S Use Assessment. b. M&S
		 * Input Pedigree. c. M&S Uncertainty Characterization. d. M&S Results Robustness. e. M&S
		 * Use/Analysis Technical Review. f. M&S Use Process/Product Management.' [M&S 31]. 'The
		 * results assessment is a product of the use phase' (Appendix E.2). A project nests one
		 * ResultsFactorAssessment per factor.
		 */
		item factorAssessments : ResultsFactorAssessment[0..*] :> resultsFactorAssessments;
	}

	abstract item resultsAssessments : MSResultsAssessment[0..*] nonunique :> items;

	/* Section 4.3: M&S use and the report of M&S results to decision makers */

	metadata def <placard> Placard {
		doc /* The warning [M&S 26] b has an M&S and its analysis results carry when the M&S was not
		 * used in accordance with the permissible uses or was used outside the domains of V&V:
		 * 'Placard the M&S and analysis results with a warning that the M&S were not used in
		 * accordance with the permissible uses or were used outside the domains of V&V and include
		 * the type of limit exceeded, the extent that the limit was exceeded, and an assessment of
		 * the consequences of this action on the M&S results.'
		 */
		attribute limitExceeded : String;
		attribute extent : String;
		attribute consequences : String;
	}

	enum def WarningOccurrence {
		doc /* The occurrences [M&S 32] has a report of M&S results warn of: 'When reporting M&S
		 * results to decision makers, explicit warnings shall be included for the following
		 * occurrences, accompanied by at least a qualitative estimate of the impact of the
		 * occurrence'.
		 */
		unachievedAcceptanceCriteria {
			doc /* a. 'Unachieved acceptance criteria (as specified in [M&S 43]).' */
		}
		violatedAssumptions {
			doc /* b. 'Violation of assumptions of the M&S used (as specified in [M&S 11]).' */
		}
		violatedLimits {
			doc /* c. 'Violation of the M&S limits (as specified in [M&S 13]).' */
		}
		executionMessages {
			doc /* d. 'Execution warning and error messages (see [M&S 27]).' */
		}
		unfavorableUseAssessment {
			doc /* e. 'Unfavorable outcomes from the assessed appropriateness of the M&S to the proposed
			 * use (described in [M&S 23]).'
			 */
		}
		setupAndUtilizationIssues {
			doc /* f. 'Issues encountered during setup and utilization of the M&S (described in [M&S
			 * 25]).'
			 */
		}
		waivers {
			doc /* g. 'Waivers to the requirements in this Standard (or the requirements specific to the
			 * M&S).'
			 */
		}
		outstandingDefects {
			doc /* h. 'Outstanding M&S defects or problems (described in [M&S 51]).' */
		}
	}

	item def ResultsWarning :> Item {
		doc /* One explicit warning of a report of M&S results [M&S 32]: the kind of occurrence warned of
		 * and 'at least a qualitative estimate of the impact of the occurrence'.
		 */
		attribute kind : WarningOccurrence;
		attribute impact : String[0..1];
	}

	abstract item resultsWarnings : ResultsWarning[0..*] nonunique :> items;

	enum def UncertaintyEstimateKind {
		doc /* The three ways [M&S 33] has a report include an estimate of results uncertainty: 'When
		 * reporting M&S results to decision makers, an estimate of results uncertainty, as defined
		 * in [M&S 29 (1)-(3)], shall be included in one of the following ways'.
		 */
		quantitative {
			doc /* a. 'A quantitative estimate of the uncertainty in the M&S results'. */
		}
		qualitative {
			doc /* b. 'A qualitative description of the uncertainty in the M&S results'. */
		}
		notAvailable {
			doc /* c. 'A clear statement that no quantitative estimate or qualitative description of
			 * uncertainty is available.'
			 */
		}
	}

	part def MSResultsReport :> Document {
		doc /* A report of M&S results to decision makers (section 4.3.8): 'more information than just
		 * the results are needed to provide a decision maker with a more complete understanding of
		 * the situation. This includes (1) the best estimate of the results, (2) a statement on the
		 * uncertainty in the results, (3) objective, credibility supporting, assessments of the M&S
		 * capabilities and M&S results (Appendix E), (4) explicit caveats accompanying the results
		 * (e.g., the use of the M&S in violation of its assumptions or M&S limits), and (5) the
		 * risks associated with accepting the results of the M&S.' A project specialises it, nests
		 * its MSCapabilityAssessment [M&S 50] and MSResultsAssessment [M&S 35] and one
		 * ResultsWarning per occurrence of [M&S 32], and states the features below.
		 */
		:>> title default = "M&S Results Report";
		attribute uncertaintyEstimate : UncertaintyEstimateKind[0..1] {
			doc /* The way the estimate of results uncertainty is included [M&S 33]. */
		}
		attribute uncertaintyProcess : String[0..1] {
			doc /* 'A description of the processes used to obtain the estimate of uncertainty' [M&S 34].
			 */
		}
		attribute technicalReviewFindings : String[0..1] {
			doc /* 'The findings from technical reviews accomplished in regard to the development,
			 * management (control), and use of the M&S' [M&S 36]. The Standard does not require
			 * technical reviews.
			 */
		}
		attribute qualifications : String[0..1] {
			doc /* 'The qualifications of the developers of the M&S and the users, operators, or analysts
			 * involved in producing the results from the M&S, including, but not limited to, their
			 * relevant education, training, and experience' [M&S 37].
			 */
		}
		attribute riskAcceptance : String[0..1] {
			doc /* 'The assessment of, and rationale for, acceptance of the risks associated with the
			 * results from the M&S-based analysis as determined in [M&S 49]' [M&S 39].
			 */
		}
	}

	abstract part resultsReports : MSResultsReport[0..*] nonunique :> parts;

	/* Section 4 and Appendix A: the requirements and the Requirements Identification Matrix */

	enum def MSRequirementArea {
		doc /* The three groups section 4 arranges its requirements and recommendations in. */
		msProgrammatics {
			doc /* Section 4.1, M&S Programmatics. */
		}
		msDevelopment {
			doc /* Section 4.2, M&S Development. */
		}
		msUse {
			doc /* Section 4.3, M&S Use. */
		}
	}

	requirement def MSRequirement :> RequirementCheck {
		doc /* One [M&S n] requirement of NASA-STD-7009B: 'Verifiable requirement statements are
		 * designated by the acronym "M&S" (Models and Simulations), numbered, and indicated by the
		 * word "shall." This Standard contains 43 requirements.' (section 1.2.4). The documentation
		 * of each definition in MSRequirements is the requirement statement; `id`, `section` and
		 * `description` are its identifier and the Section and Description columns of Table 1,
		 * `rationale` the rationale section 4 gives it. A project states the applicability of each
		 * by annotating a usage of the definition with `@Applicability`.
		 */
		attribute id : String;
		attribute section : String;
		attribute description : String;
		attribute area : MSRequirementArea;
		attribute rationale : String;
	}

	abstract requirement msRequirements : MSRequirement[0..*] nonunique :> requirementChecks;

	requirement def MSRecommendation :> RequirementCheck {
		doc /* One recommendation of section 4 of NASA-STD-7009B: '"should" denotes a good practice and
		 * is recommended but not required' (section 1.2.5). The documentation of each definition in
		 * MSRecommendations is the recommendation as Table 2 words it; `id` is the section and
		 * item letter, `section` and `description` the Section and Description of Table 2.
		 */
		attribute id : String;
		attribute section : String;
		attribute description : String;
		attribute area : MSRequirementArea;
	}

	abstract requirement msRecommendations : MSRecommendation[0..*] nonunique :> requirementChecks;

	package MSRequirements {
		doc /* The 43 requirements of NASA-STD-7009B, one definition each, named by their identifier
		 * and in the order of section 4 and Table 1. The numbering is the Standard's: identifiers
		 * were kept across revisions, so they are neither consecutive nor in section order.
		 * Section 4.2.3 gives [M&S 15] and [M&S 16] one rationale, and section 4.2.5 gives [M&S
		 * 17] and [M&S 18] one; each definition carries it.
		 */

		requirement def <'M&S 40'> MS40 :> MSRequirement {
			doc /* A record of the intended use of M&S shall be maintained. */
			:>> id = "M&S 40";
			:>> section = "4.1.1.1";
			:>> description = "General M&S Programmatics";
			:>> area = MSRequirementArea::msProgrammatics;
			:>> rationale = "The intended use is a general statement of the aspects of an RWS included in the M&S, what the M&S does, the intended domain of validation, and a description of results expected from the M&S, and a description of the decision(s) the M&S results will inform.";
		}

		requirement def <'M&S 6'> MS6 :> MSRequirement {
			doc /* A record of the assessed criticality associated with M&S use shall be maintained.
			 */
			:>> id = "M&S 6";
			:>> section = "4.1.1.2";
			:>> description = "General M&S Programmatics";
			:>> area = MSRequirementArea::msProgrammatics;
			:>> rationale = "The criticality assessment ensures communication of the amount of influence an M&S has on the intended use relative to the consequences of that situation.";
		}

		requirement def <'M&S 41'> MS41 :> MSRequirement {
			doc /* A plan for the acquisition, development, operation, maintenance, and retirement
			 * of the M&S (including identifying the responsible organizations) shall be
			 * maintained.
			 */
			:>> id = "M&S 41";
			:>> section = "4.1.1.3";
			:>> description = "General M&S Programmatics";
			:>> area = MSRequirementArea::msProgrammatics;
			:>> rationale = "The M&S development and use plans are proactive bases for proceeding through the life cycle of the M&S, including when compliance with the requirements for developing and using the M&S are accomplished.";
		}

		requirement def <'M&S 42'> MS42 :> MSRequirement {
			doc /* A record of the programmatic and technical metrics for the M&S shall be
			 * maintained.
			 */
			:>> id = "M&S 42";
			:>> section = "4.1.1.4";
			:>> description = "General M&S Programmatics";
			:>> area = MSRequirementArea::msProgrammatics;
			:>> rationale = "Defined metrics for M&S products inform developers and users as to the required data/information to collect during M&S development and use to support process improvements, assessments, or other purposes.";
		}

		requirement def <'M&S 43'> MS43 :> MSRequirement {
			doc /* A record of acceptance criteria for the M&S shall be maintained, including:
			 * a. Criteria for Verification.
			 * b. Criteria for Validation.
			 * c. Criteria for Uncertainty.
			 * d. Criteria for Sensitivity.
			 * e. M&S assessment level thresholds (for the M&S capability assessment and M&S results
			 * assessment).
			 */
			:>> id = "M&S 43";
			:>> section = "4.1.1.5";
			:>> description = "General M&S Programmatics";
			:>> area = MSRequirementArea::msProgrammatics;
			:>> rationale = "Defined M&S acceptance criteria provide qualitative or quantitative information with which to judge that the M&S or related products satisfy the intended use.";
		}

		requirement def <'M&S 44'> MS44 :> MSRequirement {
			doc /* A record of information unique to the M&S necessary when (relevant to) reporting
			 * results from the M&S shall be maintained (in addition to [M&S 32-39]).
			 */
			:>> id = "M&S 44";
			:>> section = "4.1.1.6";
			:>> description = "General M&S Programmatics";
			:>> area = MSRequirementArea::msProgrammatics;
			:>> rationale = "This requirement asks for information unique to this M&S be reported in addition to the reporting requirements of section 4.3.8 in this Standard.";
		}

		requirement def <'M&S 9'> MS9 :> MSRequirement {
			doc /* A record of results of technical reviews accomplished during the life cycle of
			 * the M&S shall be maintained.
			 */
			:>> id = "M&S 9";
			:>> section = "4.1.1.7";
			:>> description = "General M&S Programmatics";
			:>> area = MSRequirementArea::msProgrammatics;
			:>> rationale = "Technical reviews show how and what aspects of M&S development or use were reviewed and the outcome of the reviews as support for acceptance of the M&S and the results from its use. This Standard does not require technical reviews; if they occur, the results are to be recorded and made available.";
		}

		requirement def <'M&S 51'> MS51 :> MSRequirement {
			doc /* A record of M&S defects or problems from discovery through closure shall be
			 * maintained.
			 */
			:>> id = "M&S 51";
			:>> section = "4.1.1.8";
			:>> description = "General M&S Programmatics";
			:>> area = MSRequirementArea::msProgrammatics;
			:>> rationale = "A record of existing defects of problems with the M&S provides necessary cautions to M&S users and recipients of M&S-based results, as well as areas for future M&S improvement.";
		}

		requirement def <'M&S 10'> MS10 :> MSRequirement {
			doc /* A record of relevant characteristics, including data, of the RWS to be modeled,
			 * including its pedigree, shall be maintained (see Data Pedigree in Appendix E).
			 */
			:>> id = "M&S 10";
			:>> section = "4.2.1.1";
			:>> description = "General M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "Establishing the relevant characteristics of the RWS is part of analyzing and decomposing the RWS and provides the basis for M&S design, conceptual validation, development, and use.";
		}

		requirement def <'M&S 45'> MS45 :> MSRequirement {
			doc /* The data sets and supporting software used in M&S development shall be
			 * maintained.
			 */
			:>> id = "M&S 45";
			:>> section = "4.2.1.2";
			:>> description = "General M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "The data sets and supporting software provide bases and support to M&S development, possible technical reviews, and re-creation or reproduction of the M&S, as needed.";
		}

		requirement def <'M&S 46'> MS46 :> MSRequirement {
			doc /* A record of the units and vector coordinate frames for all quantities and
			 * input/output variables or parameters in the M&S shall be maintained (where
			 * applicable).
			 */
			:>> id = "M&S 46";
			:>> section = "4.2.1.3";
			:>> description = "General M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "M&S units and vector coordinate frames are a required part of correlating the M&S's representation of the RWS (and are required as part of M&S V&V).";
		}

		requirement def <'M&S 11'> MS11 :> MSRequirement {
			doc /* A record of the assumptions and abstractions underlying the conceptual and
			 * implemented model, including their rationales and consequences with respect to
			 * the intended uses of the M&S, shall be maintained.
			 */
			:>> id = "M&S 11";
			:>> section = "4.2.1.4";
			:>> description = "General M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "Making abstractions and assumptions are part of the modeling process of analyzing, decomposing, and representing an RWS. Recording the abstractions and assumptions clarifies areas where the M&S differs from the RWS or where limits of implementation mechanisms may cause the M&S' response to differ from that of the RWS. This also provides an understanding needed for verification, validation, uncertainty characterization, the specification of permissible uses, and the acceptance of the M&S for a proposed use. Failure to clearly identify, understand, and communicate assumptions and abstractions introduces risks throughout development and use of the M&S.";
		}

		requirement def <'M&S 12'> MS12 :> MSRequirement {
			doc /* A record of the concepts, structures, and mathematics of the M&S (e.g.,
			 * techniques, equations solved, behaviors or states characterized, flow of
			 * execution, conceptual models) shall be maintained.
			 */
			:>> id = "M&S 12";
			:>> section = "4.2.1.5";
			:>> description = "General M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "The act of modeling encompasses making choices as to how best to represent the RWS or elements (components) thereof. These concepts, structures, and mathematics of the M&S provide the basis for the detailed design and specification of the M&S, which provide the basis for verification of the M&S.";
		}

		requirement def <'M&S 13'> MS13 :> MSRequirement {
			doc /* A record of the M&S limits (e.g., boundary conditions) shall be maintained. */
			:>> id = "M&S 13";
			:>> section = "4.2.1.6";
			:>> description = "General M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "The M&S limits provide the broadest bounds for potential M&S Use that produce results, beyond which the M&S does (or will) not function (correctly).";
		}

		requirement def <'M&S 14'> MS14 :> MSRequirement {
			doc /* A record of the permissible uses of the M&S shall be maintained. */
			:>> id = "M&S 14";
			:>> section = "4.2.1.7";
			:>> description = "General M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "Recording the permissible uses of an M&S at the end of M&S development aggregates the latest available information about the intended uses, limiting assumptions and abstractions, and V&V limits. The permissible uses are the basis for performing the use assessment of [M&S 23].";
		}

		requirement def <'M&S 48'> MS48 :> MSRequirement {
			doc /* A record of the assessed M&S capability shall be maintained according to each of
			 * the following factors:
			 * a. M&S Data Pedigree.
			 * b. M&S Verification.
			 * c. M&S Validation.
			 * d. M&S Development Technical Review.
			 * e. M&S Development Process/Product Management.
			 */
			:>> id = "M&S 48";
			:>> section = "4.2.1.8";
			:>> description = "General M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "The assessment of each M&S capability factor ensures a discussion of the achievements and shortcomings of the M&S as developed with respect to the intended use and provides rationale for either acceptance or rejection of the M&S for release and use.";
		}

		requirement def <'M&S 47'> MS47 :> MSRequirement {
			doc /* Guidance on how to use the M&S shall be maintained, including:
			 * a. Appropriate practices for:
			 * (1) Setup.
			 * (2) Execution.
			 * (3) Interfaces with other models when the M&S is used as part of either a linked or
			 * coupled model.
			 * (4) Analysis of results.
			 * b. Obsolescence criteria.
			 * c. Parameter calibrations.
			 * d. Computational requirements (e.g., required hardware/software versions, main
			 * memory, disk capacities, processor, and compilation options).
			 */
			:>> id = "M&S 47";
			:>> section = "4.2.1.9";
			:>> description = "General M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "A user's guide provides information on the appropriate application of an M&S, including M&S functions and limitations, descriptions of appropriate practices for set-up, execution, interfaces with other models when the M&S is used as part of either a linked or coupled model, and analysis of results.";
		}

		requirement def <'M&S 15'> MS15 :> MSRequirement {
			doc /* The M&S shall be verified. */
			:>> id = "M&S 15";
			:>> section = "4.2.3.1";
			:>> description = "M&S Verification";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "Verifying the M&S ensures it is implemented in accordance with its design. The domain of verification for the M&S provides operational criterion for which it is known to produce results within the capabilities for which it was constructed.";
		}

		requirement def <'M&S 16'> MS16 :> MSRequirement {
			doc /* A record of the domain of verification of the verified M&S shall be maintained.
			 */
			:>> id = "M&S 16";
			:>> section = "4.2.3.2";
			:>> description = "M&S Verification";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "Verifying the M&S ensures it is implemented in accordance with its design. The domain of verification for the M&S provides operational criterion for which it is known to produce results within the capabilities for which it was constructed.";
		}

		requirement def <'M&S 17'> MS17 :> MSRequirement {
			doc /* The M&S shall be validated. */
			:>> id = "M&S 17";
			:>> section = "4.2.5.1";
			:>> description = "M&S Validation";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "Validating the M&S ensures it produces results acceptably similar to the RWS it represents. The domain of validation for the M&S provides operational criterion for which it is known to produce results acceptably similar to the RWS it represents.";
		}

		requirement def <'M&S 18'> MS18 :> MSRequirement {
			doc /* A record of the domain of validation of the validated M&S shall be maintained. */
			:>> id = "M&S 18";
			:>> section = "4.2.5.2";
			:>> description = "M&S Validation";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "Validating the M&S ensures it produces results acceptably similar to the RWS it represents. The domain of validation for the M&S provides operational criterion for which it is known to produce results acceptably similar to the RWS it represents.";
		}

		requirement def <'M&S 19'> MS19 :> MSRequirement {
			doc /* A record of the processes and rationale for characterizing uncertainty in the
			 * referent data shall be maintained.
			 */
			:>> id = "M&S 19";
			:>> section = "4.2.7.1";
			:>> description = "Uncertainty Characterization in M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "Explaining and recording how M&S uncertainty is incorporated in, and propagated through, an M&S ensures rational consideration was given to the effects of uncertainty during M&S development.";
		}

		requirement def <'M&S 21'> MS21 :> MSRequirement {
			doc /* A record of the qualitatively described or quantitative uncertainties
			 * incorporated into the M&S shall be maintained.
			 */
			:>> id = "M&S 21";
			:>> section = "4.2.7.2";
			:>> description = "Uncertainty Characterization in M&S Development";
			:>> area = MSRequirementArea::msDevelopment;
			:>> rationale = "Characterizing and recording the uncertainties in an M&S ensure an understanding of the type and magnitude of uncertainties that propagate through an M&S and affect the results.";
		}

		requirement def <'M&S 22'> MS22 :> MSRequirement {
			doc /* A record of the proposed use(s) of the M&S shall be maintained. */
			:>> id = "M&S 22";
			:>> section = "4.3.1.1";
			:>> description = "M&S Use";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "A defined and recorded proposed use of an M&S ensures a clear communication of how the M&S is to be used, which is the subject of the use assessment required in [M&S 23].";
		}

		requirement def <'M&S 23'> MS23 :> MSRequirement {
			doc /* A record of the appropriateness of the M&S relative to its proposed use(s) shall
			 * be maintained.
			 */
			:>> id = "M&S 23";
			:>> section = "4.3.1.2";
			:>> description = "M&S Use";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "The use assessment compares the proposed uses against the permissible uses to determine if the M&S is appropriate and if the use is within or outside the known acceptable uses of the M&S.";
		}

		requirement def <'M&S 24'> MS24 :> MSRequirement {
			doc /* A record of the Input(s) to the M&S, including their pedigrees (see Input
			 * Pedigree in Appendix E of this Standard) shall be maintained.
			 */
			:>> id = "M&S 24";
			:>> section = "4.3.1.3";
			:>> description = "M&S Use";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Recorded input data, including their pedigree, ensure an understanding of scenarios or experiments run with an M&S. These, coupled with associated M&S results, provide a basis for conclusions drawn from the M&S use.";
		}

		requirement def <'M&S 25'> MS25 :> MSRequirement {
			doc /* A record of the processes and rationales for setting up and utilizing the M&S
			 * shall be maintained.
			 */
			:>> id = "M&S 25";
			:>> section = "4.3.1.4";
			:>> description = "M&S Use";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Recorded rationale for M&S setup and execution ensures evidence is available for consideration and adjustment as needed.";
		}

		requirement def <'M&S 26'> MS26 :> MSRequirement {
			doc /* Either of the following shall be performed:
			 * a. Ensure M&S uses are conducted in accordance with the permissible uses and within
			 * the domains of V&V for the M&S, or
			 * b. Placard the M&S and analysis results with a warning that the M&S were not used in
			 * accordance with the permissible uses or were used outside the domains of V&V and
			 * include the type of limit exceeded, the extent that the limit was exceeded, and an
			 * assessment of the consequences of this action on the M&S results.
			 */
			:>> id = "M&S 26";
			:>> section = "4.3.1.5";
			:>> description = "M&S Use";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Placarding M&S results if used outside established permissible uses or domains of V&V, provides a warning to the consumers of that information.";
		}

		requirement def <'M&S 27'> MS27 :> MSRequirement {
			doc /* A record of the observed warning messages, error messages, and explanations for
			 * the messages resulting from the use of the M&S shall be maintained.
			 */
			:>> id = "M&S 27";
			:>> section = "4.3.1.6";
			:>> description = "M&S Use";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Unexplained warning or error messages are a cause for concern (increase risk) in accepting the results of an M&S use.";
		}

		requirement def <'M&S 28'> MS28 :> MSRequirement {
			doc /* A record of any processes and rationale for characterizing uncertainty in the
			 * following shall be maintained:
			 * a. The input to an M&S.
			 * b. The results from an M&S.
			 * c. The quantities derived from M&S results.
			 */
			:>> id = "M&S 28";
			:>> section = "4.3.3.1";
			:>> description = "Uncertainty Characterization in M&S Use";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Recording how and why M&S uncertainties are input to, and handled in the results of, an M&S ensures a rational basis for inclusion and analysis when an M&S is used (i.e., input and results uncertainty is not simply notional).";
		}

		requirement def <'M&S 29'> MS29 :> MSRequirement {
			doc /* A record of any uncertainties (qualitatively described or quantitative) in the
			 * following shall be maintained:
			 * a. The input to an M&S.
			 * b. The results from an M&S, including MUFs incorporated into the M&S as well as
			 * factors of safety (FS) that are intended to cover uncertainties in the model or
			 * simulation.
			 * The uncertainties those factors are intended to address should also be recorded.
			 * c. The quantities derived from M&S results.
			 */
			:>> id = "M&S 29";
			:>> section = "4.3.3.2";
			:>> description = "Uncertainty Characterization in M&S Use";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Characterizing the uncertainties in the input to and results from an M&S ensures an understanding of the type and magnitude of uncertainties that propagate through an M&S and show in the results.";
		}

		requirement def <'M&S 30'> MS30 :> MSRequirement {
			doc /* A record of the extent and results of any sensitivity analyses performed with the
			 * M&S shall be maintained.
			 */
			:>> id = "M&S 30";
			:>> section = "4.3.5";
			:>> description = "M&S Sensitivity Analysis";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Recording the extent of sensitivity analyses ensures an understanding of how well the sensitivities were investigated and match the RWS, and provides an understanding of the stability of M&S results to input (scenario) perturbations.";
		}

		requirement def <'M&S 31'> MS31 :> MSRequirement {
			doc /* A record of the assessed M&S results shall be maintained according to each of the
			 * following factors:
			 * a. M&S Use Assessment.
			 * b. M&S Input Pedigree.
			 * c. M&S Uncertainty Characterization.
			 * d. M&S Results Robustness.
			 * e. M&S Use/Analysis Technical Review.
			 * f. M&S Use Process/Product Management.
			 */
			:>> id = "M&S 31";
			:>> section = "4.3.6.1";
			:>> description = "M&S Results Assessment";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "The assessment of the M&S results according to each factor ensures a discussion of the achievements and shortcomings of the M&S use beyond just the results and provides a rational basis for the appropriate interpretation/use of the M&S results with repect to the intended use.";
		}

		requirement def <'M&S 49'> MS49 :> MSRequirement {
			doc /* A record of the assessed risks associated with the use of the M&S-based analysis
			 * shall be maintained.
			 */
			:>> id = "M&S 49";
			:>> section = "4.3.6.2";
			:>> description = "M&S Results Assessment";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "The assessment of the risks associated with the M&S-based analysis ensures the open communication of possible issues with the M&S results, or use thereof, and may inform program/project risk management processes.";
		}

		requirement def <'M&S 32'> MS32 :> MSRequirement {
			doc /* When reporting M&S results to decision makers, explicit warnings shall be
			 * included for the following occurrences, accompanied by at least a qualitative
			 * estimate of the impact of the occurrence:
			 * a. Unachieved acceptance criteria (as specified in [M&S 43]).
			 * b. Violation of assumptions of the M&S used (as specified in [M&S 11]).
			 * c. Violation of the M&S limits (as specified in [M&S 13]).
			 * d. Execution warning and error messages (see [M&S 27]).
			 * e. Unfavorable outcomes from the assessed appropriateness of the M&S to the proposed
			 * use (described in [M&S 23]).
			 * f. Issues encountered during setup and utilization of the M&S (described in [M&S
			 * 25]).
			 * g. Waivers to the requirements in this Standard (or the requirements specific to the
			 * M&S).
			 * h. Outstanding M&S defects or problems (described in [M&S 51]).
			 */
			:>> id = "M&S 32";
			:>> section = "4.3.8.1";
			:>> description = "M&S Results Reporting";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Reporting warnings for negative aspects of M&S development or use helps to ensure the appropriate acceptance of M&S results, if the results are negatively impacted.";
		}

		requirement def <'M&S 33'> MS33 :> MSRequirement {
			doc /* When reporting M&S results to decision makers, an estimate of results
			 * uncertainty, as defined in [M&S 29 (1)-(3)], shall be included in one of the
			 * following ways:
			 * a. A quantitative estimate of the uncertainty in the M&S results, or
			 * b. A qualitative description of the uncertainty in the M&S results, or
			 * c. A clear statement that no quantitative estimate or qualitative description of
			 * uncertainty is available.
			 */
			:>> id = "M&S 33";
			:>> section = "4.3.8.2";
			:>> description = "M&S Results Reporting";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Reporting an estimate of uncertainty, or lack thereof, for the M&S results ensures an understanding that the results are estimates (not exact) within assessed boundaries. Results without an estimate of uncertainty may differ vastly from those shown.";
		}

		requirement def <'M&S 34'> MS34 :> MSRequirement {
			doc /* A description of the processes used to obtain the estimate of uncertainty as
			 * defined in [M&S 28 (1)-(3)] shall be included when reporting M&S results to
			 * decision makers.
			 */
			:>> id = "M&S 34";
			:>> section = "4.3.8.3";
			:>> description = "M&S Results Reporting";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Reporting the processes used to obtain the estimate of uncertainty ensures the communication of how the uncertainties in the results occurred (or accumulated).";
		}

		requirement def <'M&S 50'> MS50 :> MSRequirement {
			doc /* The outcome of the M&S capability assessment performed per [M&S 48] shall be
			 * included in reporting M&S results to decision makers.
			 */
			:>> id = "M&S 50";
			:>> section = "4.3.8.4";
			:>> description = "M&S Results Reporting";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Reporting the M&S capability assessment ensures communication of those aspects (factors) of M&S development were accomplished, which can affect the acceptance of the M&S results.";
		}

		requirement def <'M&S 35'> MS35 :> MSRequirement {
			doc /* The outcome of the M&S results assessment performed per [M&S 31] shall be
			 * included in reporting M&S results to decision makers.
			 */
			:>> id = "M&S 35";
			:>> section = "4.3.8.5";
			:>> description = "M&S Results Reporting";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Reporting the M&S results assessment ensures communication of how well those aspects (factors) of M&S use were accomplished, which can affect the acceptance of the M&S results.";
		}

		requirement def <'M&S 36'> MS36 :> MSRequirement {
			doc /* The findings from technical reviews accomplished in regard to the development,
			 * management (control), and use of the M&S shall be included in reporting M&S
			 * results to decision makers.
			 */
			:>> id = "M&S 36";
			:>> section = "4.3.8.6";
			:>> description = "M&S Results Reporting";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Reporting findings from any technical reviews ensures the communication of supportive or unsupportive information related to what aspects of the M&S effort were reviewed.";
		}

		requirement def <'M&S 37'> MS37 :> MSRequirement {
			doc /* The qualifications of the developers of the M&S and the users, operators, or
			 * analysts involved in producing the results from the M&S, including, but not
			 * limited to, their relevant education, training, and experience, shall be included
			 * in the report of M&S results to decision makers.
			 */
			:>> id = "M&S 37";
			:>> section = "4.3.8.7";
			:>> description = "M&S Results Reporting";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Reporting the qualifications of the developers, users, operators, and/or analysts involved in producing the M&S, or results therefrom, ensures clarity in qualifications. Specific topic areas that constitute qualification depends on the type and application of the M&S; the recommendations of sections 4.1.3 and 4.1.4 may be consulted.";
		}

		requirement def <'M&S 38'> MS38 :> MSRequirement {
			doc /* The records of M&S development and use as shown in Appendix A shall be included
			 * in the report of M&S results to decision makers.
			 */
			:>> id = "M&S 38";
			:>> section = "4.3.8.8";
			:>> description = "M&S Results Reporting";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Reporting M&S records ensures clear evidence of what was actually accomplished in M&S development and use.";
		}

		requirement def <'M&S 39'> MS39 :> MSRequirement {
			doc /* The assessment of, and rationale for, acceptance of the risks associated with the
			 * results from the M&S-based analysis as determined in [M&S 49] shall be included
			 * in reporting M&S results to decision makers.
			 */
			:>> id = "M&S 39";
			:>> section = "4.3.8.9";
			:>> description = "M&S Results Reporting";
			:>> area = MSRequirementArea::msUse;
			:>> rationale = "Reporting assessment of and rationale for the risks associated with the use of the M&S-based analysis ensures the open communication of possible issues with the M&S, or use thereof, and may inform program/project risk management processes.";
		}
	}

	package MSRecommendations {
		doc /* The recommendations of section 4 of NASA-STD-7009B, one definition each, identified by
		 * section and item letter and worded as in Table 2, Recommendations Adoption Matrix
		 * ('Should record ...'). The letters are those of section 4. Table 2 omits section 4.2.2
		 * item b ('Proceed with development activities in a manner that reduces risk ...') and so
		 * letters the items after it one earlier than the section does; the library keeps the
		 * section's lettering and all nine items.
		 */

		requirement def <'4.1.2a'> R4_1_2a :> MSRecommendation {
			doc /* Should record the M&S waiver processes. */
			:>> id = "4.1.2a";
			:>> section = "4.1.2";
			:>> description = "General M&S Programmatic Recommendations";
			:>> area = MSRequirementArea::msProgrammatics;
		}

		requirement def <'4.1.2b'> R4_1_2b :> MSRecommendation {
			doc /* Should record the extent to which an M&S effort exhibits the characteristics of
			 * work product management, process definition, process measurement, process
			 * control, process change, and continuous improvement, including configuration
			 * management (CM) and M&S support and maintenance.
			 */
			:>> id = "4.1.2b";
			:>> section = "4.1.2";
			:>> description = "General M&S Programmatic Recommendations";
			:>> area = MSRequirementArea::msProgrammatics;
		}

		requirement def <'4.1.3a'> R4_1_3a :> MSRecommendation {
			doc /* Should identify and record the recommended practices that apply to M&S for the
			 * program/project.
			 */
			:>> id = "4.1.3a";
			:>> section = "4.1.3";
			:>> description = "M&S Best Practices Recommendations";
			:>> area = MSRequirementArea::msProgrammatics;
		}

		requirement def <'4.1.3b'> R4_1_3b :> MSRecommendation {
			doc /* Should at a minimum, consider the recommended practices for the following areas:
			 * (1) Data and M&S input verification, validation, and pedigree.
			 * (2) An auditing method of tracking adherence to recommended practices.
			 * (3) V&V processes for the M&S.
			 * (4) Uncertainty characterization methods for the M&S.
			 * (5) Sensitivity analysis methods for the M&S.
			 * (6) Understanding of the disciplines incorporated in the M&S.
			 * (7) Analyzing and interpreting the M&S results, including documentation of inference
			 * guidelines and statistical processes used.
			 * (8) Recognizing and capturing the need for changes or improvements in the M&S.
			 * (9) Reporting procedures for results.
			 * (10) Best practices for user interface design to constrain the operation of the M&S
			 * to within its limits of operations.
			 */
			:>> id = "4.1.3b";
			:>> section = "4.1.3";
			:>> description = "M&S Best Practices Recommendations";
			:>> area = MSRequirementArea::msProgrammatics;
		}

		requirement def <'4.1.4a'> R4_1_4a :> MSRecommendation {
			doc /* Should determine the depth of required training or equivalent experience (i.e.,
			 * qualifications) for developers, operators, and analysts.
			 */
			:>> id = "4.1.4a";
			:>> section = "4.1.4";
			:>> description = "M&S Training Recommendations";
			:>> area = MSRequirementArea::msProgrammatics;
		}

		requirement def <'4.1.4b'> R4_1_4b :> MSRecommendation {
			doc /* Should record the following:
			 * (1) Training topics required for developers, operators, and analysts of M&S, which
			 * should include the following:
			 * A. The limits of operation for M&S, with implications and rationale.
			 * B. CM requirements.
			 * C. The requirements and recommendations for recording information specified in this
			 * Standard.
			 * D. How to recognize unrealistic results from simulations.
			 * E. Feedback processes to improve M&S processes and results, including providing
			 * feedback for results that are not credible, are unrealistic, or defy explanation.
			 * F. Sensitivity analysis.
			 * G. Uncertainty characterization.
			 * H. V&V.
			 * I. How to report simulation results to decision makers.
			 * J. Statistics and probability.
			 * K. Discipline-specific recommended practices. Other applicable Agency policy,
			 * procedural requirements, and standards.
			 * L. Basic modeling structures, mathematics, assumptions, and abstractions.
			 * (2) Process and criteria for verifying that training requirements are met.
			 */
			:>> id = "4.1.4b";
			:>> section = "4.1.4";
			:>> description = "M&S Training Recommendations";
			:>> area = MSRequirementArea::msProgrammatics;
		}

		requirement def <'4.2.2a'> R4_2_2a :> MSRecommendation {
			doc /* Should record the methods of uncertainty characterization and the uncertainty in
			 * any data used to develop the M&S or incorporated into the M&S.
			 */
			:>> id = "4.2.2a";
			:>> section = "4.2.2";
			:>> description = "General M&S Development Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.2b'> R4_2_2b :> MSRecommendation {
			doc /* Should proceed with development activities in a manner that reduces risk to the
			 * project and application of the M&S by limiting the definition of the modeled RWS
			 * and its environment to only include interactions relevant to the intended use of
			 * the M&S.
			 */
			:>> id = "4.2.2b";
			:>> section = "4.2.2";
			:>> description = "General M&S Development Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.2c'> R4_2_2c :> MSRecommendation {
			doc /* Should design and construct the M&S so that, in the event of a failure, messages
			 * detailing the failure mode and point of failure are provided. This feature helps
			 * to prevent the inappropriate use of potentially misleading results.
			 */
			:>> id = "4.2.2c";
			:>> section = "4.2.2";
			:>> description = "General M&S Development Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.2d'> R4_2_2d :> MSRecommendation {
			doc /* Should record updates of the M&S (e.g., solution adjustment, change of
			 * parameters, calibration, and test cases) and assign unique version identifier,
			 * description, and the justification for the updates.
			 */
			:>> id = "4.2.2d";
			:>> section = "4.2.2";
			:>> description = "General M&S Development Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.2e'> R4_2_2e :> MSRecommendation {
			doc /* Should include test cases in the CM records that span the M&S limits defined by
			 * the program or project. 'Test cases' are defined as benchmark input/output sets
			 * used to verify proper execution of the M&S.
			 */
			:>> id = "4.2.2e";
			:>> section = "4.2.2";
			:>> description = "General M&S Development Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.2f'> R4_2_2f :> MSRecommendation {
			doc /* Should provide a feedback mechanism for users to report unusual results to M&S
			 * developers or maintainers.
			 */
			:>> id = "4.2.2f";
			:>> section = "4.2.2";
			:>> description = "General M&S Development Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.2g'> R4_2_2g :> MSRecommendation {
			doc /* Should maintain (conceptual, mathematical, and computational) models,
			 * simulations, and associated records in a controlled CM system.
			 */
			:>> id = "4.2.2g";
			:>> section = "4.2.2";
			:>> description = "General M&S Development Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.2h'> R4_2_2h :> MSRecommendation {
			doc /* Should maintain the data sets and supporting software referenced in [M&S 45] and
			 * the associated records in a controlled CM system.
			 */
			:>> id = "4.2.2h";
			:>> section = "4.2.2";
			:>> description = "General M&S Development Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.2i'> R4_2_2i :> MSRecommendation {
			doc /* Should convey serious concerns about M&S to project managers (and decision
			 * makers, if appropriate) as soon as they are known.
			 */
			:>> id = "4.2.2i";
			:>> section = "4.2.2";
			:>> description = "General M&S Development Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.4a'> R4_2_4a :> MSRecommendation {
			doc /* Should record verification techniques used. NASA-HDBK-7009 has further
			 * information regarding specific verification techniques, including the order of
			 * precedence of verification prior to validation to reduce technical and
			 * programmatic risk.
			 */
			:>> id = "4.2.4a";
			:>> section = "4.2.4";
			:>> description = "M&S Verification Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.4b'> R4_2_4b :> MSRecommendation {
			doc /* Should record numerical error estimates (e.g., numerical approximations,
			 * insufficient discretization, insufficient iterative convergence, finite-precision
			 * arithmetic) for the results of the computational model.
			 */
			:>> id = "4.2.4b";
			:>> section = "4.2.4";
			:>> description = "M&S Verification Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.4c'> R4_2_4c :> MSRecommendation {
			doc /* Should record the verification status of conceptual, mathematical, and
			 * computational models.
			 */
			:>> id = "4.2.4c";
			:>> section = "4.2.4";
			:>> description = "M&S Verification Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.4d'> R4_2_4d :> MSRecommendation {
			doc /* Should record aspects of M&S not verified. */
			:>> id = "4.2.4d";
			:>> section = "4.2.4";
			:>> description = "M&S Verification Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.6a'> R4_2_6a :> MSRecommendation {
			doc /* Should record the techniques used to validate the M&S for its intended use,
			 * including the experimental design and analysis. NASA-HDBK-7009 has further
			 * information regarding specific validation techniques.
			 */
			:>> id = "4.2.6a";
			:>> section = "4.2.6";
			:>> description = "M&S Validation Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.6b'> R4_2_6b :> MSRecommendation {
			doc /* Should record the validation metrics, referents, and data sets used for M&S
			 * validation.
			 */
			:>> id = "4.2.6b";
			:>> section = "4.2.6";
			:>> description = "M&S Validation Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.6c'> R4_2_6c :> MSRecommendation {
			doc /* Should record the studies conducted, and the status and results of, M&S
			 * validation.
			 */
			:>> id = "4.2.6c";
			:>> section = "4.2.6";
			:>> description = "M&S Validation Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.6d'> R4_2_6d :> MSRecommendation {
			doc /* Should record the aspects of the M&S not validated. */
			:>> id = "4.2.6d";
			:>> section = "4.2.6";
			:>> description = "M&S Validation Recommendations";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.2.8'> R4_2_8 :> MSRecommendation {
			doc /* The responsible party should record significant physical processes, effects,
			 * scenarios, or environments not considered in the uncertainty characterization
			 * analysis.
			 */
			:>> id = "4.2.8";
			:>> section = "4.2.8";
			:>> description = "Uncertainty Characterization in M&S Development Recommendation";
			:>> area = MSRequirementArea::msDevelopment;
		}

		requirement def <'4.3.2a'> R4_3_2a :> MSRecommendation {
			doc /* Should record the relevant characteristics of the system that is the subject of
			 * the M&S-based analysis.
			 */
			:>> id = "4.3.2a";
			:>> section = "4.3.2";
			:>> description = "M&S Use Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.2b'> R4_3_2b :> MSRecommendation {
			doc /* Should record the computational M&S used (including revision numbers) in the
			 * simulation/analysis.
			 */
			:>> id = "4.3.2b";
			:>> section = "4.3.2";
			:>> description = "M&S Use Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.2c'> R4_3_2c :> MSRecommendation {
			doc /* Should record the parameter calibrations and the domain of calibration. */
			:>> id = "4.3.2c";
			:>> section = "4.3.2";
			:>> description = "M&S Use Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.2d'> R4_3_2d :> MSRecommendation {
			doc /* Should record the data sets and supporting software used in input preparation. */
			:>> id = "4.3.2d";
			:>> section = "4.3.2";
			:>> description = "M&S Use Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.2e'> R4_3_2e :> MSRecommendation {
			doc /* Should record the processes for conducting simulations and analyses for
			 * generating results reported to decision makers.
			 */
			:>> id = "4.3.2e";
			:>> section = "4.3.2";
			:>> description = "M&S Use Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.2f'> R4_3_2f :> MSRecommendation {
			doc /* Should record the versions of M&S results. */
			:>> id = "4.3.2f";
			:>> section = "4.3.2";
			:>> description = "M&S Use Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.2g'> R4_3_2g :> MSRecommendation {
			doc /* Should record the use history of M&S, in the same or similar applications, which
			 * are relevant for assessing the capability of the current M&S application and the
			 * credibility of the results obtained from its use (see Appendix E.4.3.1 in this
			 * Standard).
			 */
			:>> id = "4.3.2g";
			:>> section = "4.3.2";
			:>> description = "M&S Use Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.2h'> R4_3_2h :> MSRecommendation {
			doc /* Should record and explain all failure modes, points of failure, and messages
			 * indicating such failures.
			 */
			:>> id = "4.3.2h";
			:>> section = "4.3.2";
			:>> description = "M&S Use Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.4'> R4_3_4 :> MSRecommendation {
			doc /* Responsible parties should record significant physical processes, effects,
			 * scenarios, or environments not considered in the uncertainty characterization
			 * analysis.
			 */
			:>> id = "4.3.4";
			:>> section = "4.3.4";
			:>> description = "M&S Uncertainty Characterization in M&S Use Recommendation";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.7'> R4_3_7 :> MSRecommendation {
			doc /* Responsible parties should justify and record the M&S results assessment for each
			 * of the factors referenced in [M&S 31].
			 */
			:>> id = "4.3.7";
			:>> section = "4.3.7";
			:>> description = "M&S Results Assessment Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.9a'> R4_3_9a :> MSRecommendation {
			doc /* Should include concluding remarks stating whether the M&S results are credible
			 * enough for the actual use.
			 */
			:>> id = "4.3.9a";
			:>> section = "4.3.9";
			:>> description = "M&S Results Reporting Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.9b'> R4_3_9b :> MSRecommendation {
			doc /* Should identify how to access more detailed backup material, including high-level
			 * descriptions of the M&S used and key assumptions for limits of validity.
			 */
			:>> id = "4.3.9b";
			:>> section = "4.3.9";
			:>> description = "M&S Results Reporting Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.9c'> R4_3_9c :> MSRecommendation {
			doc /* Should place M&S results in the CM system. */
			:>> id = "4.3.9c";
			:>> section = "4.3.9";
			:>> description = "M&S Results Reporting Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.9d'> R4_3_9d :> MSRecommendation {
			doc /* Should summarize deviations from established recommended practices. */
			:>> id = "4.3.9d";
			:>> section = "4.3.9";
			:>> description = "M&S Results Reporting Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.9e'> R4_3_9e :> MSRecommendation {
			doc /* Should include dissenting technical opinions regarding the credibility of the
			 * results or any recommended actions.
			 */
			:>> id = "4.3.9e";
			:>> section = "4.3.9";
			:>> description = "M&S Results Reporting Recommendations";
			:>> area = MSRequirementArea::msUse;
		}

		requirement def <'4.3.9f'> R4_3_9f :> MSRecommendation {
			doc /* Should convey serious concerns about M&S or its use to project managers (and
			 * decision makers, if appropriate) as soon as they are known.
			 */
			:>> id = "4.3.9f";
			:>> section = "4.3.9";
			:>> description = "M&S Results Reporting Recommendations";
			:>> area = MSRequirementArea::msUse;
		}
	}

	attribute def ApplicableCode {
		doc /* The word an Applicable value writes in the 'Applicable (Enter Yes or No)' column. */
		attribute code : String;
	}

	enum def Applicable :> ApplicableCode {
		doc /* The 'Applicable (Enter Yes or No)' column of Table 1 and Table 2: 'Enter "Yes" in the
		 * "Applicable" column if the requirement is applicable to the program or project or "No" if
		 * the requirement is not applicable to the program or project.' (Appendix A.1)
		 */
		yes {
			doc /* Yes: the requirement or recommendation applies to the program or project. */
			:>> code = "Yes";
		}
		no {
			doc /* No: it does not apply, or, for a recommendation, 'the intent is to tailor' (Appendix
			 * B.1); the comments say why or how.
			 */
			:>> code = "No";
		}
	}

	metadata def <applicability> Applicability {
		doc /* One row of the Requirements Identification Matrix (Table 1) or the Recommendations
		 * Adoption Matrix (Table 2), on a usage of the requirement or recommendation: whether it
		 * is applicable, the comments, and where its record is kept. 'The "Comments" column may be
		 * used to provide specific instructions on how to apply the requirement, specify proposed
		 * tailoring, or to provide an explanation/justification when not applicable.' (Appendix
		 * A.1). Section 4.3.8.8 reads the tables as having 'columns for applicability and rationale
		 * if not applicable' and adds: 'The location of applicable/required records is also
		 * necessary for compliance.' `requirement ms40 : MSRequirements::MS40 { @Applicability {
		 * applicable = Applicable::yes; recordLocation = "..."; } }`
		 */
		:> annotatedElement : SysML::RequirementUsage;
		:> annotatedElement : SysML::RequirementDefinition;
		attribute applicable : Applicable;
		attribute comments : String[0..1];
		attribute recordLocation : String[0..1];
	}

	calc def ApplicabilityEntries :> Query {
		doc /* The requirement and recommendation usages below root that carry an Applicability
		 * annotation.
		 */
		in root : Element;
		in depth : Integer default 16;
		WhereMetadata(
			source = Descendants(source = root, maxDepth = depth),
			'metadata' = "NASASTD7009::Applicability"
		)
	}

	calc def RequirementsIdentificationRegister :> Query {
		doc /* Table 1 generated from the project model: one row per MSRequirement usage below root
		 * that carries an Applicability annotation, with the Section, Description, requirement
		 * identifier, Requirement in this Standard (the documentation of the usage's definition),
		 * Applicable and Comments columns of Table 1 and the location of the record.
		 */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(source = ApplicabilityEntries(root = root, depth = depth), type = "NASASTD7009::MSRequirement"),
			columns = (
				Column(name = "Section", path = "section"),
				Column(name = "Description", path = "description"),
				Column(name = "Req ID", path = "id"),
				Column(name = "Requirement in this Standard", path = "type.documentation"),
				Column(name = "Applicable", path = "NASASTD7009::Applicability::applicable"),
				Column(name = "Comments", path = "NASASTD7009::Applicability::comments"),
				Column(name = "Record Location", path = "NASASTD7009::Applicability::recordLocation")
			)
		)
	}

	calc def RecommendationsAdoptionRegister :> Query {
		doc /* Table 2 generated from the project model: one row per MSRecommendation usage below root
		 * that carries an Applicability annotation, with the Section, Description, Recommendation
		 * in this Standard (the documentation of the usage's definition), Applicable and Comments
		 * columns of Table 2.
		 */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(source = ApplicabilityEntries(root = root, depth = depth), type = "NASASTD7009::MSRecommendation"),
			columns = (
				Column(name = "Section", path = "id"),
				Column(name = "Description", path = "description"),
				Column(name = "Recommendation in this Standard", path = "type.documentation"),
				Column(name = "Applicable", path = "NASASTD7009::Applicability::applicable"),
				Column(name = "Comments", path = "NASASTD7009::Applicability::comments")
			)
		)
	}

	calc def CapabilityAssessmentRegister :> Query {
		doc /* The outcome of the M&S capability assessment [M&S 50]: one row per
		 * CapabilityFactorAssessment below root with its factor, assessed level, threshold and
		 * evidence.
		 */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = depth), type = "NASASTD7009::CapabilityFactorAssessment"),
			properties = ("name", "factor", "level", "threshold", "evidence"),
			tolerate = true
		)
	}

	calc def ResultsAssessmentRegister :> Query {
		doc /* The outcome of the M&S results assessment [M&S 35]: one row per ResultsFactorAssessment
		 * below root with its factor, assessed level, threshold and evidence.
		 */
		in root : Element;
		in depth : Integer default 16;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = depth), type = "NASASTD7009::ResultsFactorAssessment"),
			properties = ("name", "factor", "level", "threshold", "evidence"),
			tolerate = true
		)
	}

	abstract part def RequirementsIdentificationMatrix :> Document {
		doc /* The Requirements Identification Matrix of Appendix A for a program or project.
		 * Specialize it and add a table whose rows run RequirementsIdentificationRegister over the
		 * project's entries: `part def ProjectMatrix :> RequirementsIdentificationMatrix { part
		 * entries : Section { part matrix : Table { calc rows : RequirementsIdentificationRegister
		 * { in :>> root = Project; } } } }`. A complete matrix has one entry per requirement; the
		 * project annotates a usage of each with `@Applicability`. [M&S 38] has the records 'as
		 * shown in Appendix A' included in the report of M&S results to decision makers.
		 */
		:>> title default = "NASA-STD-7009B Requirements Identification Matrix";
	}

	abstract part def RecommendationsAdoptionMatrix :> Document {
		doc /* The Recommendations Adoption Matrix of Appendix B for a program or project: 'a listing
		 * of recommendations contained in this Standard for selection and verification if
		 * designated by programs, projects, organizations, or offices'. Specialize it and add a
		 * table whose rows run RecommendationsAdoptionRegister.
		 */
		:>> title default = "NASA-STD-7009B Recommendations Adoption Matrix";
	}

	view def RequirementsIdentificationView :> GridView {
		doc /* Table of the requirement and recommendation usages that carry an Applicability
		 * annotation.
		 */
		filter @Applicability;
		render asElementTable;
	}

	view def CriticalityView :> GridView {
		doc /* Table of the elements that carry a CriticalityAssessment annotation. */
		filter @CriticalityAssessment;
		render asElementTable;
	}

	/* Semantic metadata: the user-defined keywords of the library */

	metadata def <capabilityAssessment> MSCapabilityAssessmentMetadata :> SemanticMetadata {
		doc /* The `#capabilityAssessment` keyword: classifies an item definition or item usage as
		 * an M&S capability assessment by applying MSCapabilityAssessment and the
		 * `capabilityAssessments` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = capabilityAssessments meta SysML::Usage;
	}

	metadata def <capabilityFactor> CapabilityFactorAssessmentMetadata :> SemanticMetadata {
		doc /* The `#capabilityFactor` keyword: classifies an item definition or item usage as the
		 * assessment of one M&S capability factor by applying CapabilityFactorAssessment and
		 * the `capabilityFactorAssessments` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = capabilityFactorAssessments meta SysML::Usage;
	}

	metadata def <resultsAssessment> MSResultsAssessmentMetadata :> SemanticMetadata {
		doc /* The `#resultsAssessment` keyword: classifies an item definition or item usage as an
		 * M&S results assessment by applying MSResultsAssessment and the `resultsAssessments`
		 * base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = resultsAssessments meta SysML::Usage;
	}

	metadata def <resultsFactor> ResultsFactorAssessmentMetadata :> SemanticMetadata {
		doc /* The `#resultsFactor` keyword: classifies an item definition or item usage as the
		 * assessment of one M&S results factor by applying ResultsFactorAssessment and the
		 * `resultsFactorAssessments` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = resultsFactorAssessments meta SysML::Usage;
	}

	metadata def <resultsWarning> ResultsWarningMetadata :> SemanticMetadata {
		doc /* The `#resultsWarning` keyword: classifies an item definition or item usage as a
		 * warning of a report of M&S results by applying ResultsWarning and the
		 * `resultsWarnings` base usage at once.
		 */
		:> annotatedElement : SysML::ItemDefinition;
		:> annotatedElement : SysML::ItemUsage;
		:>> baseType = resultsWarnings meta SysML::Usage;
	}

	metadata def <resultsReport> MSResultsReportMetadata :> SemanticMetadata {
		doc /* The `#resultsReport` keyword: classifies a part definition or part usage as a report
		 * of M&S results to decision makers by applying MSResultsReport and the
		 * `resultsReports` base usage at once.
		 */
		:> annotatedElement : SysML::PartDefinition;
		:> annotatedElement : SysML::PartUsage;
		:>> baseType = resultsReports meta SysML::Usage;
	}

	metadata def <msRequirement> MSRequirementMetadata :> SemanticMetadata {
		doc /* The `#msRequirement` keyword: classifies a requirement definition or requirement
		 * usage as an [M&S n] requirement by applying MSRequirement and the `msRequirements`
		 * base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = msRequirements meta SysML::Usage;
	}

	metadata def <msRecommendation> MSRecommendationMetadata :> SemanticMetadata {
		doc /* The `#msRecommendation` keyword: classifies a requirement definition or requirement
		 * usage as a recommendation of the Standard by applying MSRecommendation and the
		 * `msRecommendations` base usage at once.
		 */
		:> annotatedElement : SysML::RequirementDefinition;
		:> annotatedElement : SysML::RequirementUsage;
		:>> baseType = msRecommendations meta SysML::Usage;
	}
}
