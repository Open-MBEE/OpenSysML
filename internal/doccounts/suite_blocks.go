package doccounts

import "strconv"

// The generated suite blocks, each one figure or one sentence of figures, named
// by the page and the sentence carrying it.
const (
	readmeTierParserBlock      = "tier-behavioral-parser"
	readmeTierCalcBlock        = "tier-calc-evaluation"
	readmeTierActionBlock      = "tier-action-execution"
	readmeTierStateBlock       = "tier-state-machine"
	readmeTestSuiteBlock       = "test-suite"
	readmeConformanceBlock     = "conformance-passing"
	inventoryConformanceBlock  = "inventory-conformance"
	inventoryRobustnessBlock   = "inventory-robustness"
	inventoryRuntimeTestsBlock = "inventory-runtime-tests"
	inventoryGoldenASTsBlock   = "inventory-golden-asts"
	inventoryTracesBlock       = "inventory-traces"
	inventoryNegativesBlock    = "inventory-negatives"
	inventoryGRPCBlock         = "inventory-grpc"
	inventoryTestsBlock        = "inventory-tests"
	lspTestsBlock              = "lsp-tests"
)

// suiteBlocks lists the suite figures' consumers: the README and the compliance map.
func suiteBlocks() []Block {
	var blocks []Block
	for _, name := range []string{
		readmeTierParserBlock, readmeTierCalcBlock, readmeTierActionBlock, readmeTierStateBlock,
		readmeTestSuiteBlock, readmeConformanceBlock,
	} {
		blocks = append(blocks, Block{Path: ReadmePath, Name: name})
	}
	for _, name := range []string{
		inventoryConformanceBlock, inventoryRobustnessBlock, inventoryRuntimeTestsBlock,
		inventoryGoldenASTsBlock, inventoryTracesBlock, inventoryNegativesBlock,
		inventoryGRPCBlock, inventoryTestsBlock, lspTestsBlock,
	} {
		blocks = append(blocks, Block{Path: SpecCompliancePath, Name: name})
	}
	return blocks
}

// suiteBlockTemplateTexts render one span each, within the line carrying the markers.
var suiteBlockTemplateTexts = map[string]string{
	readmeTierParserBlock:  "{{.Suite.GoldenASTs}} golden ASTs, {{.Suite.Negatives}} negative tests",
	readmeTierCalcBlock:    "conformance gate: {{.Suite.CalcTierPassing}} calc/constraint/requirement/satisfy cases passing",
	readmeTierActionBlock:  "{{.Suite.ActionPassing}} conformance cases passing",
	readmeTierStateBlock:   "{{.Suite.StatePassing}} conformance cases passing",
	readmeTestSuiteBlock:   "{{.Suite.TestFunctions}} top-level `Test` functions (counted from the `_test.go` files, as `go test ./...` runs them) covering parsers, semantics, runtime (actions, states, instances, operators, validation). Behavioral robustness: {{.Suite.GoldenASTs}} golden ASTs, {{.Suite.Negatives}} negatives, {{.Suite.ConformanceCases}} conformance cases, {{.Suite.TracesDefault}} golden traces, {{.Suite.Robustness}} runtime robustness cases, {{.Suite.GRPCConformance}} gRPC conformance cases and {{.Suite.GRPCRobustness}} gRPC robustness cases.",
	readmeConformanceBlock: "{{.Suite.ConformancePassing}}/{{.Suite.ConformanceCases}} conformance cases passing",

	inventoryConformanceBlock:  "{{.Suite.ConformanceCases}} conformance cases ({{if .Suite.AllPassing}}all passing{{else}}{{.Suite.ConformancePassing}} passing, {{.Suite.KnownFailures}} listed in `known_failures.txt`{{end}}: {{.Suite.ConformanceBreakdown}})",
	inventoryRobustnessBlock:   "{{.Suite.Robustness}} runtime robustness cases (first-level subtests of `TestRuntimeRobustness`)",
	inventoryRuntimeTestsBlock: "{{.Suite.RuntimeTestFunctions}} runtime test functions (the top-level tests `go test -v ./internal/core/runtime` reports)",
	inventoryGoldenASTsBlock:   "{{.Suite.GoldenASTs}} golden AST fixtures ({{.Suite.GoldenSysML}} SysML, {{.Suite.GoldenKerML}} KerML)",
	inventoryTracesBlock:       "{{.Suite.TracesDefault}} golden execution traces under the default schedule ({{.Suite.TraceBreakdown}}), and {{.Suite.TracesPolicy}} more `.trace.golden` files pinning a case under a named policy, `<case>.declared` or `<case>.seed-<n>`",
	inventoryNegativesBlock:    "{{.Suite.Negatives}} negative parser subtests (first-level subtests of `TestNegative`; {{.Suite.NegativesPrefixed}} across the `TestNegative*` functions, {{.Suite.NegativesKerML}} of them KerML, and {{.Suite.NegativesAll}} across every `*Negative*` parser test)",
	inventoryGRPCBlock:         "{{.Suite.GRPCConformance}} gRPC conformance cases and {{.Suite.GRPCRobustness}} gRPC robustness cases",
	inventoryTestsBlock:        "{{.Suite.TestFunctions}} top-level `Test` functions across the module",
	lspTestsBlock:              "{{.Suite.LSPTestFunctions}} top-level `Test` functions in `internal/lsp`",
}

// suiteFigures are the suite counts spelt as the templates print them.
type suiteFigures struct {
	TestFunctions        string
	RuntimeTestFunctions string
	LSPTestFunctions     string
	ConformanceCases     int
	ConformancePassing   int
	KnownFailures        int
	AllPassing           bool
	ConformanceBreakdown string
	CalcTierPassing      string
	ActionPassing        string
	StatePassing         string
	GoldenASTs           int
	GoldenSysML          int
	GoldenKerML          int
	Negatives            int
	NegativesPrefixed    int
	NegativesKerML       int
	NegativesAll         int
	Robustness           int
	TracesDefault        int
	TracesPolicy         int
	TraceBreakdown       string
	GRPCConformance      int
	GRPCRobustness       int
}

func figuresOf(counts SuiteCounts) suiteFigures {
	conformance := counts.Conformance
	return suiteFigures{
		TestFunctions:        Thousands(counts.TestFunctions),
		RuntimeTestFunctions: Thousands(counts.RuntimeTestFunctions),
		LSPTestFunctions:     Thousands(counts.LSPTestFunctions),
		ConformanceCases:     conformance.Cases,
		ConformancePassing:   conformance.Passing,
		KnownFailures:        conformance.Cases - conformance.Passing,
		AllPassing:           conformance.Cases == conformance.Passing,
		ConformanceBreakdown: Breakdown(conformance.Prefixes),
		CalcTierPassing:      passingOf(conformance, "calc", "constraint", "requirement", "satisfy"),
		ActionPassing:        passingOf(conformance, "action"),
		StatePassing:         passingOf(conformance, "state"),
		GoldenASTs:           counts.GoldenASTs.Total,
		GoldenSysML:          counts.GoldenASTs.SysML,
		GoldenKerML:          counts.GoldenASTs.KerML,
		Negatives:            counts.Negatives.Table,
		NegativesPrefixed:    counts.Negatives.Prefixed,
		NegativesKerML:       counts.Negatives.KerML,
		NegativesAll:         counts.Negatives.All,
		Robustness:           counts.Robustness,
		TracesDefault:        counts.Traces.Default,
		TracesPolicy:         counts.Traces.Policy,
		TraceBreakdown:       Breakdown(counts.Traces.Prefixes),
		GRPCConformance:      counts.GRPCConformance,
		GRPCRobustness:       counts.GRPCRobustness,
	}
}

// passingOf spells the passing cases of the prefixes: `160`, or `158 of 160`
// while known_failures.txt lists some.
func passingOf(counts ConformanceCounts, prefixes ...string) string {
	total, passing := counts.Of(prefixes...), counts.PassingOf(prefixes...)
	if passing == total {
		return strconv.Itoa(total)
	}
	return strconv.Itoa(passing) + " of " + strconv.Itoa(total)
}
