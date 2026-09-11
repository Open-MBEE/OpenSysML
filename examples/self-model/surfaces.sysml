// What sits on top of the pipeline: the interactive surfaces, the client
// libraries that reach the service, and the artifacts the tool writes out.
package OpenSysMLSurfaces {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;
    private import OpenSysMLPipeline::*;

    abstract part def Interface :> CodeUnit {
        attribute entrypoint : String;
        attribute protocol : String;
        port pipeline : RequestPort;
    }

    #Surface part def Repl :> Interface {
        attribute :>> goPackage = "internal/repl";
        attribute :>> entrypoint = "cmd/sysml";
        attribute :>> protocol = "terminal";
        attribute stateful : Boolean = true;
        attribute adoptsInstancesOnReanalysis : Boolean = true;
    }

    // Every capability the server announces in its initialize result; the
    // rendering requests are the experimental extension the VS Code client uses.
    #Surface part def LanguageServer :> Interface {
        attribute :>> goPackage = "internal/lsp";
        attribute :>> entrypoint = "cmd/sysml-lsp";
        attribute :>> protocol = "lsp-stdio";
        attribute incrementalSync : Boolean = true;
        attribute publishesQuickFixes : Boolean = true;
        attribute hover : Boolean = true;
        attribute definition : Boolean = true;
        attribute findReferences : Boolean = true;
        attribute documentSymbols : Boolean = true;
        attribute workspaceSymbols : Boolean = true;
        attribute completion : Boolean = true;
        attribute formatting : Boolean = true;
        attribute rename : Boolean = true;
        attribute semanticTokens : Boolean = true;
        attribute experimentalRendering : Boolean = true;
        in item findings : Diagnostic;
        out item tokens : SemanticToken;
        out item fixes : QuickFix;
    }

    // The schema every transport and every client is generated from. Only the
    // Go stubs are checked in beside it; the other stubs live with their clients.
    part def ApiSchema :> CodeUnit {
        attribute :>> goPackage = "api/proto";
        attribute schemaFile : String = "api/proto/sysml.proto";
        attribute service : String = "SysMLService";
        attribute rpcCount : Integer = 21;
        attribute rpcs : String = "GetServerInfo, ParseFile, ParseSources, GetSymbol, GetDiagnostics, Evaluate, Instantiate, ExecuteAction, ExecuteState, Convert, ApplyEdits, VerifyConstraint, VerifyRequirement, VerifySatisfaction, EvaluateCalc, RunAnalysis, RunSweep, ListEngines, Query, RunDocumentQuery, RenderDocument";
        attribute generatedGo : String = "api/proto/sysml.pb.go, api/proto/sysml_grpc.pb.go, api/proto/protoconnect/sysml.connect.go";
        attribute generator : String = "buf";
    }

    // One hand-written service behind two generated transports: gRPC over
    // HTTP/2 with reflection, and Connect over plain HTTP for browsers.
    #Surface part def ModelService :> Interface {
        attribute :>> goPackage = "internal/grpc";
        attribute :>> entrypoint = "cmd/sysml-grpc";
        attribute :>> protocol = "grpc, connect";
        attribute reflectionEnabled : Boolean = true;
        attribute handWrittenService : Boolean = true;
        attribute connectAdapter : String = "internal/grpc/connect.go";
        attribute stdioTransport : String = "internal/stdiorpc";
        part schema : ApiSchema;
    }

    // The same service over a pipe: a process that embeds the binary talks to
    // it without a network listener.
    #Surface part def StdioService :> Interface {
        attribute :>> goPackage = "internal/stdiorpc";
        attribute :>> entrypoint = "cmd/sysml-grpc";
        attribute :>> protocol = "stdio";
    }

    // The non-interactive side of the REPL binary: convert, query, render.
    #Surface part def CommandLine :> Interface {
        attribute :>> goPackage = "internal/repl";
        attribute :>> entrypoint = "cmd/sysml";
        attribute :>> protocol = "flags";
        attribute exitStatusIsContract : Boolean = true;
    }

    // A client library: hand-written idiomatic code over stubs generated from
    // the schema, the stubs checked in at generatedStubs.
    abstract part def Client :> CodeUnit {
        attribute implementationLanguage : String;
        attribute generatedFrom : String = "api/proto";
        attribute generatedStubs : String;
        port service : RequestPort;
    }

    part def PythonClient :> Client {
        attribute :>> goPackage = "clients/python";
        attribute :>> implementationLanguage = "python";
        attribute :>> generatedStubs = "clients/python/opensysml/proto";
    }

    part def NodeClient :> Client {
        attribute :>> goPackage = "clients/node";
        attribute :>> implementationLanguage = "typescript";
        attribute :>> generatedStubs = "clients/node/src/generated";
    }

    part def RustClient :> Client {
        attribute :>> goPackage = "clients/rust";
        attribute :>> implementationLanguage = "rust";
        attribute :>> generatedStubs = "clients/rust/opensysml/src/proto";
    }

    part def JavaClient :> Client {
        attribute :>> goPackage = "clients/java";
        attribute :>> implementationLanguage = "java";
        attribute :>> generatedStubs = "clients/java/opensysml-client/src/main/java/org/openmbee/opensysml/proto";
    }

    // In the same process by default, over the wire on request: the Go client
    // wraps the generated Go stubs rather than being generated itself.
    part def GoClient :> Client {
        attribute :>> goPackage = "client/opensysml";
        attribute :>> implementationLanguage = "go";
        attribute :>> generatedStubs = "api/proto";
        attribute inProcess : Boolean = true;
    }

    part def VsCodeExtension :> CodeUnit {
        attribute :>> goPackage = "editors/vscode";
        attribute speaksTo : String = "sysml-lsp";
        port languageClient : RequestPort;
    }

    // --- What an editor reaches ------------------------------------------

    // Semantic-token classes, in the order the LSP legend lists them.
    part def Highlighter :> CodeUnit {
        attribute :>> goPackage = "internal/core/highlight";
        attribute tokenClasses : Integer = 15;
        in item tree : SyntaxTree;
        out item tokens : SemanticToken;
    }

    // A fix carries the edit that applies it and points at the diagnostic it
    // answers; a diagnostic with no unambiguous repair offers none.
    part def QuickFixer :> CodeUnit {
        attribute :>> goPackage = "internal/core/quickfix";
        attribute editsAttached : Boolean = true;
        in item findings : Diagnostic;
        out item fixes : QuickFix;
    }

    // The nearest declared names for one that did not resolve.
    part def Suggester :> CodeUnit {
        attribute :>> goPackage = "internal/core/suggest";
        attribute rankedByDistance : Boolean = true;
        in item findings : Diagnostic;
    }

    // Span-based rewriting of source text. The result is re-parsed and
    // re-analysed, and an edit that would leave the model unreadable is refused.
    part def SourceEditor :> CodeUnit {
        attribute :>> goPackage = "internal/core/edit";
        attribute operations : String = "set value, rename, add member, delete";
        attribute preservesUntouchedText : Boolean = true;
        attribute reanalysesResult : Boolean = true;
        in item text : SourceText;
        out item edits : SourceEdit;
    }

    // Re-indents the token stream, so comments and error nodes survive a save.
    part def Formatter :> CodeUnit {
        attribute :>> goPackage = "internal/core/format";
        attribute worksOnTokens : Boolean = true;
        in item text : SourceText;
    }

    // A document name and a span, carried from a declaration to whatever is
    // derived from it; a synthetic artifact carries the unlocated origin.
    part def Provenance :> CodeUnit {
        attribute :>> goPackage = "internal/core/provenance";
        attribute keyedByDocumentAndSpan : Boolean = true;
        attribute consumers : String = "internal/core/docir, internal/core/view, internal/core/queryplan";
        out item origins : SourceSpan;
    }

    // Diagnostics feed fixes and suggestions; edits go back in as source text.
    part def EditorPipeline {
        part highlighter : Highlighter;
        part fixes : QuickFixer;
        part suggestions : Suggester;
        part editor : SourceEditor;
        part formatter : Formatter;

        port findingsIn : DiagnosticPort;
        port textOut : ArtifactPort;

        flow of Diagnostic from fixes.findings to suggestions.findings;
        flow of SourceEdit from editor.edits to formatter.text;
    }

    // --- What the tool writes out ----------------------------------------

    // A rendering kind the standard view definitions name.
    part def RenderingKind {
        attribute kind : String;
        attribute supported : Boolean default = true;
    }

    // Every kind in StandardViewDefinitions is recognized, so a view stating
    // one that is not produced is told so rather than rendered as another.
    part def ViewEngine :> CodeUnit {
        attribute :>> goPackage = "internal/core/view";
        attribute recognizedKinds : Integer = 8;
        attribute supportedKinds : Integer = 6;
        part tree : RenderingKind { attribute :>> kind = "tree"; }
        part interconnection : RenderingKind { attribute :>> kind = "interconnection"; }
        part stateDiagram : RenderingKind { attribute :>> kind = "state"; }
        part actionDiagram : RenderingKind { attribute :>> kind = "action"; }
        part textual : RenderingKind {
            attribute :>> kind = "textual";
            attribute :>> supported = false;
        }
        part table : RenderingKind { attribute :>> kind = "table"; }
        part sequence : RenderingKind { attribute :>> kind = "sequence"; }
        part geometry : RenderingKind {
            attribute :>> kind = "geometry";
            attribute :>> supported = false;
        }
        in item types : TypeTable;
        in item origins : SourceSpan;
        out item diagram : DocumentTree;
    }

    part def QueryCompiler :> CodeUnit {
        attribute :>> goPackage = "internal/core/queryplan";
        attribute immutablePlans : Boolean = true;
    }

    part def QueryExecutor :> CodeUnit {
        attribute :>> goPackage = "internal/core/queryexec";
        attribute orderedRows : Boolean = true;
        out item rows : QueryRows;
    }

    // A document is written in the notation, not in a template language: the
    // planner compiles a document definition into an immutable plan holding
    // compiled query programs.
    part def DocumentPlanner :> CodeUnit {
        attribute :>> goPackage = "internal/core/docplan";
        attribute libraryFile : String =
            "internal/core/libs/stdlib/OpenSysML Libraries/DocumentQueries.sysml";
        attribute immutablePlans : Boolean = true;
        attribute contentKinds : String = "section, paragraph, table, list, diagram";
        in item rows : QueryRows;
        out item plan : DocumentPlan;
    }

    // Evaluating a plan gives one tree, carrying provenance on every node and
    // knowing nothing about the backend that will write it out.
    part def DocumentIR :> CodeUnit {
        attribute :>> goPackage = "internal/core/docir";
        attribute backendAgnostic : Boolean = true;
        attribute provenancePerNode : Boolean = true;
        attribute resolvesCrossDocumentRefs : Boolean = true;
        in item plan : DocumentPlan;
        in item origins : SourceSpan;
        out item tree : DocumentTree;
    }

    part def MarkdownBackend :> CodeUnit {
        attribute :>> goPackage = "internal/core/docrender";
        attribute form : String = "markdown";
        attribute needsExternalTool : Boolean = false;
        in item tree : DocumentTree;
    }

    // The only stage that leaves the process: the tree becomes prepared
    // Markdown, its diagrams become images, and a converter makes the PDF.
    part def PdfBackend :> CodeUnit {
        attribute :>> goPackage = "internal/docpdf";
        attribute form : String = "pdf";
        attribute engines : String = "weasyprint, pandoc, prince";
        attribute diagramTool : String = "mmdc";
        attribute needsExternalTool : Boolean = true;
        attribute reportsMissingTool : Boolean = true;
        in item tree : DocumentTree;
    }

    // Textual notation both ways, experimental RDF, and SysML v1 XMI read and
    // migrated. Three formats, nine names: kerml and text are the notation,
    // turtle and rdf are Turtle, uml is XMI as Eclipse UML2 writes it and
    // mdzip is XMI in a zip archive.
    part def Exporter :> CodeUnit {
        attribute :>> goPackage = "internal/core/export";
        attribute formats : String = "sysml, ttl, xmi";
        attribute formatNames : String = "kerml, mdzip, rdf, sysml, text, ttl, turtle, uml, xmi";
        attribute roundTrips : Boolean = true;
        out item graph : RdfGraph;
    }

    part def OslcQuery :> CodeUnit {
        attribute :>> goPackage = "internal/core/query";
        attribute dialect : String = "oslc";
    }

    // The document path, from a query in the model to a rendered artifact.
    // One plan, one tree, two backends: the form is chosen at the end, so
    // Markdown and PDF cannot disagree about the document's content.
    part def DocumentPipeline {
        part provenance : Provenance;
        part views : ViewEngine;
        part queries : QueryCompiler;
        part rows : QueryExecutor;
        part plans : DocumentPlanner;
        part ir : DocumentIR;
        part markdown : MarkdownBackend;
        part pdf : PdfBackend;

        port modelIn : ArtifactPort;
        port artifactOut : ArtifactPort;

        flow of SourceSpan from provenance.origins to views.origins;
        flow of SourceSpan from provenance.origins to ir.origins;
        flow of QueryRows from rows.rows to plans.rows;
        flow of DocumentPlan from plans.plan to ir.plan;
        flow of DocumentTree from ir.tree to markdown.tree;
        flow of DocumentTree from ir.tree to pdf.tree;
        flow of DocumentTree from views.diagram to pdf.tree;
    }

    // One document rendered, and the one branch in it: the form.
    action def RenderDocument {
        attribute form : String = "markdown";
        attribute toolsPresent : Boolean = true;

        in item model : SourceText;
        out item artifact : DocumentTree;

        first start;

        action runQueries;
        action compilePlan;
        action evaluatePlan;
        action writeMarkdown;
        action renderDiagrams;
        action convertPdf;
        action reportMissingTool;

        succession first start then runQueries;
        succession first runQueries then compilePlan;
        succession first compilePlan then evaluatePlan;
        succession first evaluatePlan then formGate;

        decide formGate;
        if form == "markdown" then writeMarkdown;
        else renderDiagrams;

        // A missing converter is reported, never a half-written document.
        succession first renderDiagrams then toolGate;

        decide toolGate;
        if toolsPresent then convertPdf;
        else reportMissingTool;
    }

    requirement def DocumentsCarryTheirProvenance {
        doc /* Every rendered node can be traced back to the element it came from,
             * and both backends read the same tree. */
        subject documents : DocumentPipeline;
        require constraint {
            documents.ir.provenancePerNode and documents.ir.backendAgnostic
                and documents.plans.immutablePlans
        }
    }

    // --- Conformance oracles ---------------------------------------------

    // Harnesses that referee this implementation against a reference; each
    // holds a committed baseline that must move deliberately. An advisory
    // oracle reports drift; a gating one fails the build on it.
    abstract part def ConformanceOracle :> CodeUnit {
        attribute reference : String;
        attribute baseline : String;
        attribute gating : Boolean default = false;
    }

    #Oracle part def RejectionOracle :> ConformanceOracle {
        attribute :>> goPackage = "cmd/pilot-reject";
        attribute :>> reference = "pilot validator";
        attribute :>> baseline = "docs/project/pilot-rejection-baseline.json";
    }

    #Oracle part def DifferentialOracle :> ConformanceOracle {
        attribute :>> goPackage = "cmd/pilot-diff";
        attribute :>> reference = "pilot validator";
        attribute :>> baseline = "docs/project/pilot-differential-baseline.json";
    }

    #Oracle part def XpectOracle :> ConformanceOracle {
        attribute :>> goPackage = "cmd/pilot-xpect";
        attribute :>> reference = "pilot Xpect suites";
        attribute :>> baseline = "docs/project/pilot-xpect-baseline.json";
    }

    #Oracle part def ExecutionReferee :> ConformanceOracle {
        attribute :>> goPackage = "cmd/pilot-exec-diff";
        attribute :>> reference = "pilot expression evaluator";
        attribute :>> baseline = "docs/project/pilot-execution-referee.md";
    }

    #Oracle part def GrammarCoverage :> ConformanceOracle {
        attribute :>> goPackage = "cmd/grammar-coverage";
        attribute :>> reference = "OMG Xtext grammars";
        attribute :>> baseline = "docs/project/grammar-coverage-baseline.json";
    }

    #Oracle part def DocCounts :> ConformanceOracle {
        attribute :>> goPackage = "cmd/doc-counts";
        attribute :>> reference = "the measured repository";
        attribute :>> baseline = "docs/project/spec-compliance.md";
    }

    // The bundled standard library must parse clean; the gate is in the suite.
    #Oracle part def StdlibConformanceGate :> ConformanceOracle {
        attribute :>> goPackage = "internal/core/libs";
        attribute :>> reference = "OMG standard library release";
        attribute :>> baseline = "internal/core/libs/stdlib";
        attribute :>> gating = true;
    }

    // The committed library snapshot must be the one a fresh build writes.
    #Oracle part def StdlibSnapshotGate :> ConformanceOracle {
        attribute :>> goPackage = "internal/core/libs/gensnapshot";
        attribute :>> reference = "the bundled library files";
        attribute :>> baseline = "internal/core/libs/stdlib.snapshot";
        attribute :>> gating = true;
    }

    // What the oracles lean on: a pinned reference, an errata overlay for the
    // defects in OMG-published material, and the census the status page reads.
    part def OraclePin :> CodeUnit {
        attribute :>> goPackage = "internal/baseline";
        attribute recordsPin : Boolean = true;
        attribute recordsBridgeSource : Boolean = true;
        attribute recordsCorpusProvenance : Boolean = true;
    }

    part def Errata :> CodeUnit {
        attribute :>> goPackage = "internal/errata";
        attribute overlaysPublishedMaterial : Boolean = true;
    }

    part def StatusCensus :> CodeUnit {
        attribute :>> goPackage = "internal/doccounts";
        attribute countsStatusMarkers : Boolean = true;
    }

    part def ConformanceInfrastructure {
        part pins : OraclePin;
        part errata : Errata;
        part census : StatusCensus;
    }

    // --- The whole tool ---------------------------------------------------

    part def Toolchain {
        part pipeline : AnalysisPipeline;
        part documents : DocumentPipeline;
        part editing : EditorPipeline;

        part repl : Repl;
        part lsp : LanguageServer;
        part service : ModelService;
        part stdio : StdioService;
        part cli : CommandLine;

        part goClient : GoClient;
        part python : PythonClient;
        part node : NodeClient;
        part rust : RustClient;
        part java : JavaClient;
        part vscode : VsCodeExtension;

        part exporter : Exporter;
        part oslc : OslcQuery;

        part rejection : RejectionOracle;
        part differential : DifferentialOracle;
        part xpect : XpectOracle;
        part execution : ExecutionReferee;
        part grammar : GrammarCoverage;
        part counts : DocCounts;
        part stdlib : StdlibConformanceGate;
        part snapshotGate : StdlibSnapshotGate;
        part conformance : ConformanceInfrastructure;

        connection replDrives : ArtifactChannel connect repl.pipeline to pipeline.textIn;
        connection lspDrives : ArtifactChannel connect lsp.pipeline to pipeline.textIn;
        connection serviceDrives : ArtifactChannel connect service.pipeline to pipeline.textIn;
        connection cliDrives : ArtifactChannel connect cli.pipeline to pipeline.textIn;

        connection goCalls : ArtifactChannel connect goClient.service to service.pipeline;
        connection pythonCalls : ArtifactChannel connect python.service to service.pipeline;
        connection nodeCalls : ArtifactChannel connect node.service to service.pipeline;
        connection rustCalls : ArtifactChannel connect rust.service to service.pipeline;
        connection javaCalls : ArtifactChannel connect java.service to service.pipeline;
        connection editorSpeaksLsp : ArtifactChannel connect vscode.languageClient to lsp.pipeline;

        connection documentsRead : ArtifactChannel connect pipeline.textIn to documents.modelIn;
        connection findingsToEditors : DiagnosticChannel connect pipeline.diagnosticsOut to editing.findingsIn;
        connection editsBack : ArtifactChannel connect editing.textOut to pipeline.textIn;
    }

    part opensysml : Toolchain;

    requirement documentsAreTraceable : DocumentsCarryTheirProvenance {
        subject documents = opensysml.documents;
    }

    requirement def ViewsSayWhatTheyCannotRender {
        doc /* Every standard rendering kind is recognized, and the ones not
             * produced say so instead of being drawn as something else. */
        subject views : ViewEngine;
        require constraint {
            views.recognizedKinds == 8 and views.supportedKinds == 6
                and not views.textual.supported and not views.geometry.supported
                and views.sequence.supported
        }
    }

    requirement viewsAreHonest : ViewsSayWhatTheyCannotRender {
        subject views = opensysml.documents.views;
    }
}
