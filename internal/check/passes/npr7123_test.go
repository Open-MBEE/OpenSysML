package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// npr7123Model wraps body in a package importing the NPR7123 library.
func npr7123Model(body string) string {
	return "package M {\n\tprivate import NPR7123::*;\n\tprivate import NPR7123::SERequirements::*;" + body + "\n}"
}

var npr7123Codes = []string{
	CodeNPR7123ReviewMissingCriteria,
	CodeNPR7123ReviewMissingProduct,
	CodeNPR7123ComplianceNoJustification,
	CodeNPR7123MatrixMissingRequirement,
	CodeNPR7123SEMPMissingProcess,
}

// A life-cycle review states entrance and success criteria; one stating
// neither, or only one kind, is reported once, and a peer review is not.
func TestNPR7123ReviewMissingCriteria(t *testing.T) {
	src := npr7123Model(`
		#lifeCycleReview action bare { :>> kind = ReviewKind::TRR; }
		#lifeCycleReview action entranceOnly {
			:>> kind = ReviewKind::TRR;
			requirement e1 : ReviewCriteria::TRR::Entrance1 :> entranceCriteria;
		}
		#lifeCycleReview action complete {
			:>> kind = ReviewKind::TRR;
			requirement e1 : ReviewCriteria::TRR::Entrance1 :> entranceCriteria;
			requirement s1 : ReviewCriteria::TRR::Success1 :> successCriteria;
		}
		#peerReview action peer;`)
	w8dWantLines(t, src, CodeNPR7123ReviewMissingCriteria, 4, 5)
}

// A review of a kind with section 5.2.2.2 minimum products is reported for each product it does
// not state or states below the required maturity; a kind without minimum products is not.
func TestNPR7123ReviewMissingMinimumProduct(t *testing.T) {
	src := npr7123Model(`
		#lifeCycleReview action cdr {
			:>> kind = ReviewKind::CDR;
			requirement e1 : ReviewCriteria::CDR::Entrance1 :> entranceCriteria;
			requirement s1 : ReviewCriteria::CDR::Success1 :> successCriteria;
		}
		#lifeCycleReview action pdr {
			:>> kind = ReviewKind::PDR;
			requirement e1 : ReviewCriteria::PDR::Entrance1 :> entranceCriteria;
			requirement s1 : ReviewCriteria::PDR::Success1 :> successCriteria;
			#designSolution item design :> reviewedProducts { :>> maturity = WorkProductMaturity::preliminary; }
			#integrationPlan item plan :> reviewedProducts { :>> maturity = WorkProductMaturity::update; }
			#verificationAndValidationPlan item vv :> reviewedProducts { :>> maturity = WorkProductMaturity::preliminary; }
		}
		#lifeCycleReview action trr {
			:>> kind = ReviewKind::TRR;
			requirement e1 : ReviewCriteria::TRR::Entrance1 :> entranceCriteria;
			requirement s1 : ReviewCriteria::TRR::Success1 :> successCriteria;
		}`)
	w8dWantLines(t, src, CodeNPR7123ReviewMissingProduct, 4, 15)
}

// A compliance entry marked tailored or notApplicable states a justification;
// a fully compliant one needs none.
func TestNPR7123ComplianceMissingJustification(t *testing.T) {
	src := npr7123Model(`
		part entries {
			requirement a : SE07 { @Compliance { comply = ComplyKind::fullyCompliant; } }
			requirement b : SE08 { @Compliance { comply = ComplyKind::tailored; justification = "deviation D-1"; } }
			requirement c : SE09 { @Compliance { comply = ComplyKind::tailored; } }
			requirement d : SE10 { @Compliance { comply = ComplyKind::notApplicable; } }
		}`)
	w8dWantLines(t, src, CodeNPR7123ComplianceNoJustification, 7, 8)
}

// A compliance matrix is reported while an active SE requirement has no
// compliance entry in its document, and once for the whole matrix.
func TestNPR7123ComplianceMatrixMissingRequirement(t *testing.T) {
	src := npr7123Model(`
		part entries {
			requirement a : SE07 { @Compliance { comply = ComplyKind::fullyCompliant; } }
		}
		part def Matrix :> ComplianceMatrix;
		part matrix : Matrix;`)
	w8dWantLines(t, src, CodeNPR7123MatrixMissingRequirement, 8)
}

// A SEMP addresses all 17 processes, through nested usages at any depth; one
// that misses any is reported, and a SEMP stated as a reviewed product is not.
func TestNPR7123SEMPMissingProcess(t *testing.T) {
	src := npr7123Model(`
		#semp part partial : SEMP {
			#stakeholderExpectationsDefinition action a;
		}
		#semp part full : SEMP {
			#stakeholderExpectationsDefinition action p1; #technicalRequirementsDefinition action p2;
			#logicalDecomposition action p3; #designSolutionDefinition action p4;
			#productImplementation action p5; #productIntegration action p6;
			#productVerification action p7; #productValidation action p8;
			#productTransition action p9; #technicalPlanning action p10;
			part management {
				#requirementsManagement action p11; #interfaceManagement action p12;
				#technicalRiskManagement action p13; #configurationManagement action p14;
				#technicalDataManagement action p15; #technicalAssessment action p16;
				#decisionAnalysis action p17;
			}
		}
		#lifeCycleReview action srr {
			:>> kind = ReviewKind::SRR;
			requirement e1 : ReviewCriteria::SRR::Entrance1 :> entranceCriteria;
			requirement s1 : ReviewCriteria::SRR::Success1 :> successCriteria;
			#semp part sempAtSRR :> reviewedProducts { :>> maturity = WorkProductMaturity::baseline; }
			#requirementSet item reqs :> reviewedProducts { :>> maturity = WorkProductMaturity::baseline; }
			#humanSystemsIntegrationApproach item hsi :> reviewedProducts { :>> maturity = WorkProductMaturity::baseline; }
		}`)
	w8dWantLines(t, src, CodeNPR7123SEMPMissingProcess, 4)
}

// A model that never uses the library, or a workspace package merely named
// NPR7123, gets no finding.
func TestNPR7123NeedsTheLibrary(t *testing.T) {
	plain := `package M {
		action def LifeCycleReview; action review : LifeCycleReview;
		part def SEMP; part semp : SEMP;
	}`
	namesake := `package NPR7123 {
		action def LifeCycleReview; action review : LifeCycleReview;
		part def SEMP; part semp : SEMP;
	}`
	for _, src := range []string{plain, namesake} {
		for _, code := range npr7123Codes {
			w8dWantLines(t, src, code)
		}
	}
}

// A review inherits its project definition's criteria and products; a product nested without subsetting
// reviewedProducts is not stated, and the best-matured of two stated products satisfies the minimum in any order.
func TestNPR7123ReviewReadsInheritedAndSubsettingFeatures(t *testing.T) {
	src := npr7123Model(`
		action def ProjectCDR :> LifeCycleReview {
			:>> kind = ReviewKind::CDR;
			requirement e1 : ReviewCriteria::CDR::Entrance1 :> entranceCriteria;
			requirement s1 : ReviewCriteria::CDR::Success1 :> successCriteria;
			#designSolution item design :> reviewedProducts { :>> maturity = WorkProductMaturity::baseline; }
			#integrationPlan item plan :> reviewedProducts { :>> maturity = WorkProductMaturity::baseline; }
			#verificationAndValidationPlan item vv :> reviewedProducts { :>> maturity = WorkProductMaturity::baseline; }
		}
		#lifeCycleReview action inherited : ProjectCDR;
		#lifeCycleReview action nestedOnly {
			:>> kind = ReviewKind::TRR;
			requirement e1 : ReviewCriteria::TRR::Entrance1;
			requirement s1 : ReviewCriteria::TRR::Success1 :> successCriteria;
		}
		#lifeCycleReview action twoDesigns {
			:>> kind = ReviewKind::PDR;
			requirement e1 : ReviewCriteria::PDR::Entrance1 :> entranceCriteria;
			requirement s1 : ReviewCriteria::PDR::Success1 :> successCriteria;
			#designSolution item draft :> reviewedProducts { :>> maturity = WorkProductMaturity::initial; }
			#designSolution item design :> reviewedProducts { :>> maturity = WorkProductMaturity::preliminary; }
			#integrationPlan item plan :> reviewedProducts { :>> maturity = WorkProductMaturity::preliminary; }
			#verificationAndValidationPlan item vv { :>> maturity = WorkProductMaturity::preliminary; }
		}`)
	w8dWantLines(t, src, CodeNPR7123ReviewMissingCriteria, 13)
	w8dWantLines(t, src, CodeNPR7123ReviewMissingProduct, 18, 24)
}

// A SEMP a review nests without subsetting reviewedProducts is the SEMP itself and is audited.
func TestNPR7123SEMPNestedInReviewWithoutSubsettingIsAudited(t *testing.T) {
	src := npr7123Model(`
		#lifeCycleReview action srr {
			:>> kind = ReviewKind::SRR;
			requirement e1 : ReviewCriteria::SRR::Entrance1 :> entranceCriteria;
			requirement s1 : ReviewCriteria::SRR::Success1 :> successCriteria;
			#semp part standIn :> reviewedProducts;
			#semp part theSEMP;
		}`)
	w8dWantLines(t, src, CodeNPR7123SEMPMissingProcess, 9)
}

// The SEMP walk visits each feature once, so a structure that reaches itself
// through a feature's type (next : Node) ends instead of recursing without bound.
func TestNPR7123SEMPWalkEndsOnARecursiveStructure(t *testing.T) {
	src := npr7123Model(`
		part def Node { ref part next : Node[0..1]; }
		#semp part plan {
			part head : Node;
		}`)
	w8dWantLines(t, src, CodeNPR7123SEMPMissingProcess, 5)
}

// A stand-in SEMP a review definition, not only a review usage, lists under
// reviewedProducts is the product at the review and is not audited.
func TestNPR7123SEMPStandInInAReviewDefinitionIsNotAudited(t *testing.T) {
	src := npr7123Model(`
		#lifeCycleReview action def ReviewTemplate {
			:>> kind = ReviewKind::SRR;
			requirement e1 : ReviewCriteria::SRR::Entrance1 :> entranceCriteria;
			requirement s1 : ReviewCriteria::SRR::Success1 :> successCriteria;
			#semp part standIn :> reviewedProducts;
		}`)
	w8dWantLines(t, src, CodeNPR7123SEMPMissingProcess)
}

// A below-maturity product inherited from a definition in another document is
// reported at the review, naming the definition, not at the foreign declaration.
func TestNPR7123InheritedProductIsReportedAtTheReview(t *testing.T) {
	templates := `package Templates {
	private import NPR7123::*;
	#lifeCycleReview action def CDRTemplate {
		:>> kind = ReviewKind::CDR;
		requirement e : ReviewCriteria::CDR::Entrance1 :> entranceCriteria;
		requirement s : ReviewCriteria::CDR::Success1 :> successCriteria;
		#designSolution item design :> reviewedProducts {
			:>> maturity = WorkProductMaturity::preliminary;
		}
	}
}`
	project := `package Project {
	private import NPR7123::*;
	private import Templates::*;
	action cdr : CDRTemplate;
}`
	idx := newTestIndex()
	troot := parser.New(source.New("templates.sysml", []byte(templates))).ParseFile()
	proot := parser.New(source.New("project.sysml", []byte(project))).ParseFile()
	idx.AddDocument("templates.sysml", troot)
	idx.AddDocument("project.sysml", proot)
	var got []diag.Diagnostic
	for _, d := range Analyze("project.sysml", proot, nil, idx) {
		if d.Code == CodeNPR7123ReviewMissingProduct && strings.Contains(d.Message, "DesignSolution") {
			got = append(got, d)
		}
	}
	if len(got) != 1 || w8dLine(project, got[0].Span) != 4 || !strings.Contains(got[0].Message, "inherited from CDRTemplate") {
		t.Fatalf("design solution diagnostics = %+v, want one at the review on line 4 naming CDRTemplate", got)
	}
}

// The library's tables are read only for a document stating a review, a SEMP or a
// compliance matrix: reading them for every document of a large model dominated its analysis.
func TestNPR7123ReadsLibraryTablesOnlyWhenUsed(t *testing.T) {
	loaded := func(src string) bool {
		t.Helper()
		root := parser.New(source.New("<t>", []byte(src))).ParseFile()
		idx := newTestIndex()
		idx.AddDocument("<t>", root)
		a := newNPR7123Audit(NewContext("<t>", idx, nil))
		if a == nil {
			t.Fatal("the bundled NPR7123 library is not loaded")
		}
		a.check(idx.DocumentRoot("<t>"))
		return a.tablesLoaded
	}
	if loaded("package M { part def D; part p : D; }") {
		t.Error("a model that never uses the library read its tables")
	}
	if loaded(npr7123Model("\n\tpart entries { requirement c : SE09 { @Compliance { comply = ComplyKind::tailored; } } }")) {
		t.Error("a model with compliance entries alone read the library's tables")
	}
	if !loaded(npr7123Model("\n\t#lifeCycleReview action bare { :>> kind = ReviewKind::TRR; }")) {
		t.Error("a model stating a life-cycle review did not read the library's tables")
	}
}
