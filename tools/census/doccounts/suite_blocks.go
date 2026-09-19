package doccounts

// The suite blocks, each one figure or one sentence of figures, named by the
// page and the sentence carrying it. Only the README's conformance-passing block
// is committed: it moves with known_failures.txt alone. The rest move with every
// fixture and test, so the documentation build renders them (SiteBlocks).
const (
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

// suiteBlocks lists the committed suite blocks.
func suiteBlocks() []Block {
	return []Block{{Path: ReadmePath, Name: readmeConformanceBlock}}
}

// siteSuiteBlocks lists the suite blocks the documentation build renders: the
// compliance map's test inventory and the LSP coverage line.
func siteSuiteBlocks() []Block {
	var blocks []Block
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
	readmeConformanceBlock: "{{if .Suite.AllPassing}}every conformance case passing{{else}}every conformance case passing but the {{.Suite.KnownFailures}} `known_failures.txt` lists{{end}}",

	inventoryConformanceBlock:  "{{.Suite.ConformanceCases}} conformance cases ({{if .Suite.AllPassing}}all passing{{else}}{{.Suite.ConformancePassing}} passing, {{.Suite.KnownFailures}} listed in `known_failures.txt`{{end}}: {{.Suite.ConformanceBreakdown}})",
	inventoryRobustnessBlock:   "{{.Suite.Robustness}} runtime robustness cases (first-level subtests across the `TestRuntimeRobustness*` functions)",
	inventoryRuntimeTestsBlock: "{{.Suite.RuntimeTestFunctions}} runtime test functions (the top-level tests `go test -v ./internal/exec/runtime` reports)",
	inventoryGoldenASTsBlock:   "{{.Suite.GoldenASTs}} golden AST fixtures ({{.Suite.GoldenSysML}} SysML, {{.Suite.GoldenKerML}} KerML)",
	inventoryTracesBlock:       "{{.Suite.TracesDefault}} golden execution traces under the default schedule ({{.Suite.TraceBreakdown}}), and {{.Suite.TracesPolicy}} more `.trace.golden` files pinning a case under a named policy, `<case>.declared` or `<case>.seed-<n>`",
	inventoryNegativesBlock:    "{{.Suite.Negatives}} negative parser subtests (first-level subtests of `TestNegative`; {{.Suite.NegativesPrefixed}} across the `TestNegative*` functions, {{.Suite.NegativesKerML}} of them KerML, and {{.Suite.NegativesAll}} across every `*Negative*` parser test)",
	inventoryGRPCBlock:         "{{.Suite.GRPCConformance}} gRPC conformance cases and {{.Suite.GRPCRobustness}} gRPC robustness cases (first-level subtests across the `TestGRPCRobustness*` functions)",
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
