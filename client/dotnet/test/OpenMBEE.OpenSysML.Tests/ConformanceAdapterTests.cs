using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Reflection.Emit;
using System.Text;
using System.Threading.Tasks;
using Google.Protobuf;
using Google.Protobuf.Reflection;
using OpenMBEE.OpenSysML.Conformance;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance adapters answer a scenario through the public typed API: the request the typed call
/// sends is the one the scenario decoded, the answer rendered is the complete answer the service gave,
/// a failure reported in band is rendered in band, a refusal keeps its status, a request the API cannot
/// express is skipped with its stable reason and sends nothing, and nothing reaches the service raw.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceAdapterTests
{
    private const string Hash = "h";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    /// <summary>For each model RPC, a request setting every field the API can send and an answer setting every field it reads.</summary>
    public static TheoryData<ClientWire, string> Exchanges()
    {
        var data = new TheoryData<ClientWire, string>();
        foreach (var wire in new[] { ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson })
        {
            foreach (var name in Complete.Keys)
            {
                data.Add(wire, name);
            }
        }

        return data;
    }

    public static TheoryData<ClientWire, string> Failures()
    {
        var data = new TheoryData<ClientWire, string>();
        foreach (var wire in new[] { ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson })
        {
            foreach (var name in InBand.Keys)
            {
                data.Add(wire, name);
            }
        }

        return data;
    }

    public static TheoryData<string, string, string> UnexpressibleRequests => new()
    {
        { "ParseFile", "{}", "the public API always names a source, so it cannot send a request naming none" },
        { "ParseFile", """{"file_path": "/m/a.sysml", "language": "kerml"}""", "the public API reads a file's notation from its extension, so it cannot name a language for one" },
        { "ParseFile", """{"file_path": ""}""", "the public API names a file by a path that is not empty" },
        { "ParseFile", """{"content": "package P;", "language": "fortran"}""", "the public API names a language as a SourceLanguage, which has none spelled \"fortran\"" },
        { "ParseFile", """{"content": "package P;", "content_hash": "abc"}""", "the public API has no way to send sysml.ParseFileRequest.content_hash" },
        { "ParseSources", """{"documents": [{}]}""", "the public API always names a document's source, so it cannot send a document naming none" },
        { "ParseSources", """{"documents": [{"file_path": "/m/a.sysml", "name": "a"}]}""", "the public API names a file document by its path alone, so it cannot give one a name or a language" },
        { "ParseSources", """{"documents": [{"file_path": ""}]}""", "the public API names a file document by a path that is not empty" },
        { "GetSymbol", """{"model_hash": " ", "symbol_id": "Demo::Vehicle"}""", "the public API names a model by a hash that is not blank" },
        { "Evaluate", """{"expression": "2 + 2"}""", "the public API names a model by a hash that is not blank" },
        { "Query", """{"model_hash": "h"}""", "the public API always names a query, so it cannot send a request naming none" },
        { "Query", """{"model_hash": "h", "query": {}, "oslc_query": "sysml:type=PartUsage"}""", "the public API names a query in one form, structured or OSLC text, so it cannot send both" },
        { "Query", """{"model_hash": "h", "query": {"where": {"primitive": {"property": "@type", "value": ["PartUsage"]}}}}""", "the public API names a comparison's operator, which query.where.primitive does not" },
        { "Query", """{"model_hash": "h", "query": {"where": {}}}""", "the public API names a constraint's form, which query.where does not" },
        { "Query", """{"model_hash": "h", "query": {"where": {"composite": {"constraint": [{"primitive": {"property": "name", "operator": "PRIMITIVE_OPERATOR_EQUAL", "value": ["a"]}}]}}}}""", "the public API names a composite's operator, which query.where.composite does not" },
        { "Query", """{"model_hash": "h", "query": {"where": {"composite": {"operator": "COMPOSITE_OPERATOR_OR"}}}}""", "the public API cannot hold query.where.composite as spelled: A composite combines at least two constraints, got none." },
        { "Query", """{"model_hash": "h", "query": {"where": {"composite": {"operator": "COMPOSITE_OPERATOR_AND", "constraint": [{"primitive": {"property": "name", "operator": "PRIMITIVE_OPERATOR_EQUAL", "value": ["a"]}}]}}}}""", "the public API cannot hold query.where.composite as spelled: A composite combines at least two constraints, got one; write the constraint on its own." },
        { "Query", """{"model_hash": "h", "query": {"where": {"primitive": {"property": "name", "operator": "PRIMITIVE_OPERATOR_INSTANCE_OF", "value": ["Usage"]}}}}""", "the public API cannot hold query.where.primitive as spelled: An InstanceOf comparison compares @type, the element's metaclass, not \"name\"." },
        { "Query", """{"model_hash": "h", "query": {"where": {"composite": {"operator": "COMPOSITE_OPERATOR_AND", "constraint": [{"primitive": {"property": "multiplicityLower", "operator": "PRIMITIVE_OPERATOR_GREATER", "value": ["1", "2"]}}]}}}}""", "the public API cannot hold query.where.composite.constraint[0].primitive as spelled: A Greater comparison compares against exactly one value, not 2." },
        { "Query", """{"model_hash": "h", "query": {"where": {"primitive": {"operator": "PRIMITIVE_OPERATOR_EQUAL", "value": ["a"]}}}}""", "the public API cannot hold query.where.primitive as spelled: The value cannot be an empty string." },
        { "RunDocumentQuery", """{"model_hash": "h"}""", "the public API names a document query by a qualified name that is not empty" },
        { "RunDocumentQuery", """{"model_hash": "h", "query_id": "Q", "bindings": [{"values": [{"string_value": "x"}]}]}""", "the public API cannot hold bindings[0] as spelled: The value cannot be an empty string." },
        { "RunDocumentQuery", """{"model_hash": "h", "query_id": "Q", "bindings": [{"parameter": "p", "values": [{"infinity": true}]}]}""", "the public API cannot hold bindings[0] as spelled: The value p[0] is infinity, which queries answer and are never bound to." },
        { "RunDocumentQuery", """{"model_hash": "h", "query_id": "Q", "bindings": [{"parameter": "p", "values": [{"element_type": "PartUsage"}]}]}""", "the public API cannot hold bindings[0] as spelled: The value p[0] is an element named by no qualified name, which a query cannot be bound to." },
        { "RunDocumentQuery", """{"model_hash": "h", "query_id": "Q", "bindings": [{"parameter": "p", "values": [{"object": {}}]}]}""", "the public API cannot hold bindings[0] as spelled: The value p[0] is an object named by neither a path nor an id, which ids count up from 1." },
        { "RunDocumentQuery", """{"model_hash": "h", "query_id": "Q", "bindings": [{"parameter": "p", "values": [{"verdict": {"verdict": "holds"}}]}]}""", "the public API binds no value as bindings[0].values[0] spells it (Verdict), which queries answer and are never bound to" },
        { "RunDocumentQuery", """{"model_hash": "h", "query_id": "Q", "bindings": [{"parameter": "p", "values": [{"object": {"path": "car", "element": {"string_value": "x"}}}]}]}""", "the public API holds an object's element as an element value, which bindings[0].values[0].object.element is not" },
        { "RunDocumentQuery", """{"model_hash": "h", "query_id": "Q", "bindings": [{"parameter": "p", "values": [{"rational_value": {"numerator": "2", "denominator": "4"}}]}]}""", "the public API writes a rational in lowest terms over a positive denominator, which bindings[0].values[0]'s 2/4 is not" },
        { "RenderDocument", """{"model_hash": "h"}""", "the public API names a document by a qualified name that is not empty" },
        { "RenderDocument", """{"model_hash": "h", "document_id": "D", "form": "markdown"}""", "the public API sends the Markdown form as the empty form it defaults to" },
        { "RenderDocument", """{"model_hash": "h", "document_id": "D", "form": "pdf"}""", "the public API names a form as a DocumentForm, which has none spelled \"pdf\"" },
        { "RenderView", """{"model_hash": "h"}""", "the public API names a view by a name that is not empty" },
        { "RenderView", """{"model_hash": "h", "view": "V", "ports": "minimal"}""", "the public API sends minimal ports as the empty selection they default to" },
        { "RenderView", """{"model_hash": "h", "view": "V", "ports": "all"}""", "the public API names ports as a RenderViewPorts, which has none spelled \"all\"" },
        { "RenderView", """{"model_hash": "h", "view": "V", "depth": -1}""", "RenderViewOptions.Depth is -1, which draws no level; leave it null for the view's own depth." },
        { "VerifyConstraint", """{"symbol_id": "C"}""", "the public API names a model by a hash that is not blank" },
        { "EvaluateCalc", """{"model_hash": "h", "arguments": [{"function": {"calc_id": "F::Scaler::scale", "self_id": 3}}]}""", "The value at arguments[0] is the function F::Scaler::scale closing over instance(3), which lives only within the response that held it." },
        { "EvaluateCalc", """{"model_hash": "h", "arguments": [{"measurement_ref": {"unit": "m"}}]}""", "the public API holds a measurement reference with its reduction, which arguments[0] has none of" },
        { "VerifyRequirement", """{"model_hash": "h", "named_arguments": {"x": {"undetermined": {}}}}""", "The value at named_arguments[\"x\"] is undetermined, which is a result the model leaves open, never an argument." },
        { "RunSweep", """{"model_hash": "h", "ranges": [{"parameter": "b", "end": {"int_value": 1}}]}""", "the public API holds a sweep range with both endpoints, which ranges[0] does not state" },
        { "RunAnalysis", """{"model_hash": "h", "schedule": "explore", "tool_dry_run": true}""", "AnalysisOptions.ToolDryRun previews one run of the case, which an exploration makes none of; leave it false." },
        { "ExecuteAction", """{"model_hash": "h", "inputs": {"x": {"unset": true}}}""", "The value at inputs[\"x\"] is unset, which is what a feature with no value reads as, never an argument; leave the argument out instead." },
        { "ExecuteAction", """{"model_hash": "h", "inputs": {"x": {"big_int_value": "+9223372036854775808"}}}""", "the public API writes an integer in canonical digits, which inputs[\"x\"]'s \"+9223372036854775808\" is not" },
        { "ExecuteAction", """{"model_hash": "h", "inputs": {"x": {"rational_value": {"numerator": "2", "denominator": "4"}}}}""", "the public API writes a rational in lowest terms over a positive denominator, which inputs[\"x\"]'s 2/4 is not" },
        { "ExecuteAction", """{"model_hash": "h", "inputs": {"x": {}}}""", "the public API holds no value as inputs[\"x\"] spells it (None)" },
        { "ExecuteAction", """{"model_hash": "h", "inputs": {"x": {"undetermined": {"reason": "open"}}}}""", "The value at inputs[\"x\"] is undetermined, which is a result the model leaves open, never an argument." },
        { "ExecuteAction", """{"model_hash": "h", "inputs": {"x": {"sequence": {"elements": [{"quantity": {"unit": "m"}}]}}}}""", "the public API holds a quantity with its magnitude, which inputs[\"x\"].sequence[0].quantity has none of" },
        { "CheckBehavior", """{"model_hash": "h", "action_symbol_id": "A", "inputs": {"x": {"vector": {"components": [{"bool_value": true}]}}}}""", "the public API holds a vector's components as numbers, which inputs[\"x\"].vector[0] is not" },
        { "ExecuteState", """{"model_hash": "h", "schedule": "explore", "trace": true}""", "RunOptions.Trace asks for one run's trace, which an exploration describes none of; leave it false." },
        { "CheckBehavior", """{"model_hash": "h", "action_symbol_id": "A", "state_machine_symbol_id": "S"}""", "the public API checks one behavior, so it cannot name both an action and a state machine" },
        { "CheckBehavior", """{"model_hash": "h", "state_machine_symbol_id": "S", "inputs": {"x": {"int_value": 1}}}""", "the public API binds inputs only to an action it checks, so it cannot send them with a state machine" },
        { "RunRuns", """{"model_hash": "h", "action_symbol_id": "A", "analysis_symbol_id": "C"}""", "the public API runs one behavior, so it cannot name both an action and a Monte Carlo case" },
        { "RunRuns", """{"model_hash": "h", "action_symbol_id": "A", "subject_symbol_id": "S"}""", "the public API binds a subject and arguments only to a Monte Carlo case it runs, so it cannot send them with an action" },
        { "RunRuns", """{"model_hash": "h", "named_arguments": {"x": {"int_value": 1}}}""", "the public API binds a subject and arguments only to a Monte Carlo case it runs, so it cannot send them with an action" },
        { "RunRuns", """{"model_hash": "h", "analysis_symbol_id": "C", "observe": ["t"]}""", "the public API binds inputs, a performer and observables only to an action it runs, so it cannot send them with a Monte Carlo case" },
        { "RunRuns", """{"model_hash": "h", "analysis_symbol_id": "C", "performer_symbol_id": "P"}""", "the public API binds inputs, a performer and observables only to an action it runs, so it cannot send them with a Monte Carlo case" },
        { "RecordRun", """{"model_hash": "h", "analysis_symbol_id": "C"}""", "the public API reads every document a recording answers, so it always accepts them" },
        { "Convert", """{"to_format": "sysml"}""", "the public API always names a source, so it cannot send a request naming none" },
        { "Convert", """{"file_path": "", "to_format": "sysml"}""", "the public API names a file by a path that is not empty" },
        { "Convert", """{"model_hash": " ", "to_format": "sysml"}""", "the public API names a model by a hash that is not blank" },
        { "Convert", """{"content": "package P;", "from_format": "sysml", "to_format": "ttl", "id_form": "Uuid"}""", "the public API names an id form as an IdForm, which has none spelled \"Uuid\"" },
        { "Migrate", """{"to_format": "sysml"}""", "the public API always names a source, so it cannot send a request naming none" },
        { "Migrate", """{"file_path": "", "to_format": "sysml"}""", "the public API names a file by a path that is not empty" },
        { "Migrate", """{"file_path": "Vehicle.xmi", "to_format": "sysml"}""", "the public API requires a fully-qualified, valid migration file path" },
        { "Migrate", """{"content": "AQ==", "to_format": "sysml", "layout_path": ""}""", "MigrateOptions names an empty form, layout or image base URL; leave FromFormat, LayoutPath, LayoutContent or ImageBaseUrl null for none." },
        { "RecordRun", """{"model_hash": "h", "analysis_symbol_id": "C", "accept_documents": true, "ranges": [{"parameter": "b", "start": {"int_value": 0}}]}""", "the public API holds a sweep range with both endpoints, which ranges[0] does not state" },
        { "ApplyEdits", """{"model_hash": " ", "operations": [{"rename": {"target": "A", "new_name": "B"}}]}""", "the public API names a model by a hash that is not blank" },
        { "ApplyEdits", """{"model_hash": "h", "operations": [{"rename": {"target": "A", "new_name": "B"}}, {}]}""", "the public API names an edit's form, which operations[1] does not" },
        { "ExportGraphs", """{"model_hash": " ", "subject": "Test::race"}""", "the public API names a model by a hash that is not blank" },
        { "ExportGraphs", """{"model_hash": "h"}""", "the public API names a behavior by a qualified name that is not empty" },
        { "ImportData", """{"model_hash": "h", "imports": [{"name": "a.csv"}, {"content": "element\n"}]}""", "the public API names a data file by a name that is not empty, which imports[1] does not" },
        { "CompareResults", """{"model_hash": "h"}""", "the public API always names the results compared, so it cannot send a request naming none" },
        { "CompareResults", """{"model_hash": "h", "results": "{}", "observe": [{"stored": " ", "feature": "target.total"}]}""", "the public API names a stored observable by a name that is not blank, which observe[0] does not" },
    };

    private static Dictionary<string, (IMessage Request, IMessage Response)> Complete => new(StringComparer.Ordinal)
    {
        ["ParseFile"] = (
            new Wire.ParseFileRequest { Content = "package P;", Language = "kerml", StrictConformance = true },
            new Wire.ParseFileResponse { ModelHash = Hash, Root = CompleteWire.Symbol, Diagnostics = { CompleteWire.Diagnostic, CompleteWire.BareDiagnostic } }),
        ["ParseFile from a file"] = (
            new Wire.ParseFileRequest { FilePath = "/models/a.sysml", StrictConformance = true },
            new Wire.ParseFileResponse { ModelHash = Hash, Root = CompleteWire.Symbol }),
        ["ParseSources"] = (
            new Wire.ParseSourcesRequest
            {
                Documents =
                {
                    new Wire.SourceDocument { Content = "package A;", Name = "a.sysml", Language = "sysml" },
                    new Wire.SourceDocument { Content = "package B;" },
                    new Wire.SourceDocument { FilePath = "/models/c.sysml" },
                },
                StrictConformance = true,
                BaseModelHash = "base",
            },
            new Wire.ParseSourcesResponse
            {
                ModelHash = Hash,
                Roots = { CompleteWire.Symbol, new Wire.SymbolInfo { Kind = "RootNamespace" } },
                Diagnostics = { CompleteWire.Diagnostic },
                Affected = { "a.sysml", "/models/c.sysml" },
            }),
        ["GetSymbol"] = (
            new Wire.GetSymbolRequest { ModelHash = Hash, SymbolId = "Demo::Vehicle" },
            new Wire.SymbolResponse { Symbol = CompleteWire.Symbol }),
        ["GetDiagnostics"] = (
            new Wire.DiagnosticsRequest { ModelHash = Hash, DisableLint = { "undeclared-signal" }, EnableLint = { "rounded-real-literal" } },
            new Wire.DiagnosticsResponse { Diagnostics = { CompleteWire.Diagnostic, CompleteWire.BareDiagnostic } }),
        ["Evaluate"] = (
            new Wire.EvaluateRequest { ModelHash = Hash, Expression = "mass", ContextSymbolId = "Demo::Vehicle", SubjectSymbolId = "Demo::sedan" },
            new Wire.EvaluateResponse
            {
                Result = new Wire.Value { Sequence = new Wire.ValueSequence { Elements = { CompleteWire.Values } } },
                Diagnostics = { CompleteWire.Diagnostic },
            }),
        ["Instantiate"] = (
            new Wire.InstantiateRequest { ModelHash = Hash, SymbolId = "Demo::Vehicle" },
            new Wire.InstantiateResponse
            {
                Instance = CompleteWire.Instance,
                Instances = { CompleteWire.Instance, new Wire.Instance { Id = 10, TypeSymbolId = "Demo::Engine" } },
                Diagnostics = { CompleteWire.Diagnostic },
            }),
        ["ExecuteAction"] = (
            new Wire.ExecuteActionRequest
            {
                ModelHash = Hash,
                ActionSymbolId = "Test::addFive",
                Inputs = { SendableValues },
                Schedule = "declared",
                PerformerSymbolId = "Plant::tank",
                Advance = 3,
                Seed = 42,
                Draws = "min",
                ClockStep = 0.25,
            },
            CompleteExecutionWire.ActionRun),
        ["ExecuteAction exploring"] = (
            new Wire.ExecuteActionRequest { ModelHash = Hash, ActionSymbolId = "Test::race", Schedule = "explore:runs=8,depth=4", Advance = 0 },
            CompleteExecutionWire.ActionExploration),
        ["ExecuteState"] = (
            new Wire.ExecuteStateRequest
            {
                ModelHash = Hash,
                StateMachineSymbolId = "Test::Machine",
                Events = { "go", "stop" },
                Schedule = "seed:3",
                PerformerSymbolId = "Test::box",
                Trace = true,
                Advance = 1,
                Seed = 7,
                Draws = "random",
                ClockStep = 2,
            },
            CompleteExecutionWire.StateRun),
        ["ExecuteState exploring"] = (
            new Wire.ExecuteStateRequest { ModelHash = Hash, StateMachineSymbolId = "Test::Machine", Events = { "go" }, Schedule = "explore" },
            CompleteExecutionWire.StateExploration),
        ["CheckBehavior"] = (
            new Wire.CheckBehaviorRequest
            {
                ModelHash = Hash,
                ActionSymbolId = "Plant::Tank::fill",
                Inputs = { ["x"] = new Wire.Value { IntValue = 3 } },
                PerformerSymbolId = "Plant::tank",
                CheckProperty = { "Plant::Tank::low" },
                CheckDiverge = { "this.level" },
                CheckInput = { "x" },
                CheckAssume = { "Plant::Tank::capped" },
                CheckWitness = true,
                CheckDepth = -1,
                CheckStates = 10,
                CheckUnroll = 2,
                CheckTimeout = 0.5,
                Engine = "explore",
            },
            CompleteExecutionWire.Check),
        ["CheckBehavior of a state machine"] = (
            new Wire.CheckBehaviorRequest { ModelHash = Hash, StateMachineSymbolId = "Test::Machine", CheckProperty = { "Test::ok" } },
            new Wire.CheckBehaviorResponse { Verdict = "holds", Engine = "check" }),
        ["VerifyConstraint"] = (
            new Wire.VerifyConstraintRequest
            {
                ModelHash = Hash,
                SymbolId = "Demo::Vehicle::massLight",
                SubjectSymbolId = "Demo::sedan",
                Engine = "explore",
                Question = "holds",
                Arguments = { SendableValues.Values },
                NamedArguments = { SendableValues },
            },
            CompleteVerificationWire.Constraint),
        ["VerifyRequirement"] = (
            new Wire.VerifyRequirementRequest
            {
                ModelHash = Hash,
                SymbolId = "Demo::bounded",
                SubjectSymbolId = "Demo::sedan",
                Engine = "all",
                Question = "bogus",
                Arguments = { SendableValues.Values },
                NamedArguments = { SendableValues },
            },
            CompleteVerificationWire.Requirement),
        ["VerifySatisfaction"] = (
            new Wire.VerifySatisfactionRequest { ModelHash = Hash, SymbolId = "Demo::checks", Engine = "auto", Question = "satisfiable" },
            CompleteVerificationWire.Satisfaction),
        ["VerifySatisfaction of every assertion"] = (
            new Wire.VerifySatisfactionRequest { ModelHash = Hash },
            new Wire.VerifySatisfactionResponse()),
        ["ValidateInstance"] = (
            new Wire.ValidateInstanceRequest { ModelHash = Hash, SymbolId = "Demo::sedan", Engine = "run" },
            CompleteVerificationWire.Validation),
        ["EvaluateCalc"] = (
            new Wire.EvaluateCalcRequest { ModelHash = Hash, SymbolId = "Demo::add", Arguments = { SendableValues.Values }, Engine = "run" },
            CompleteVerificationWire.Calculation),
        ["RunAnalysis"] = (
            new Wire.RunAnalysisRequest
            {
                ModelHash = Hash,
                SymbolId = "An::CostAnalysis",
                SubjectSymbolId = "An::barge",
                Arguments = { SendableValues.Values },
                NamedArguments = { SendableValues },
                Schedule = "declared",
                Engine = "run",
                ToolDryRun = true,
            },
            CompleteVerificationWire.Analysis),
        ["RunAnalysis exploring"] = (
            new Wire.RunAnalysisRequest { ModelHash = Hash, SymbolId = "An::race", Engine = " explore " },
            CompleteVerificationWire.Exploration),
        ["RunSweep"] = (
            new Wire.RunSweepRequest
            {
                ModelHash = Hash,
                SymbolId = "Sw::CostAnalysis",
                SubjectSymbolId = "Sw::barge",
                Arguments = { SendableValues.Values },
                NamedArguments = { SendableValues },
                Ranges =
                {
                    new Wire.SweepRange { Parameter = "limit", Start = new Wire.Value { RealValue = 0 }, End = new Wire.Value { RealValue = 40 }, Step = new Wire.Value { RealValue = 20 } },
                    new Wire.SweepRange { Start = new Wire.Value { IntValue = -1 }, End = new Wire.Value { Quantity = new Wire.Quantity { IntMagnitude = 1 } } },
                },
                Samples = -2,
                Seed = ulong.MaxValue,
                Engine = "sweep",
            },
            CompleteVerificationWire.Sweep),
        ["ListEngines"] = (
            new Wire.ListEnginesRequest { Probe = true },
            CompleteVerificationWire.Engines),
        ["CheckBehavior of no behavior"] = (
            new Wire.CheckBehaviorRequest { ModelHash = Hash },
            new Wire.CheckBehaviorResponse { Verdict = "undecided" }),
        ["RunRuns"] = (
            new Wire.RunRunsRequest
            {
                ModelHash = Hash,
                ActionSymbolId = "MC::Settle",
                Inputs = { SendableValues },
                PerformerSymbolId = "MC::probe",
                Runs = 3,
                Seed = ulong.MaxValue,
                Draws = "min",
                ClockStep = 0.25,
                Observe = { "t", "mode" },
                Engine = "sweep",
            },
            CompleteSamplingWire.ActionRuns),
        ["RunRuns of a Monte Carlo case"] = (
            new Wire.RunRunsRequest
            {
                ModelHash = Hash,
                AnalysisSymbolId = "MC::Mc",
                SubjectSymbolId = "MC::probe",
                Arguments = { SendableValues.Values },
                NamedArguments = { SendableValues },
                Runs = 1,
                Seed = 0,
                Draws = "average",
                ClockStep = 2,
                Engine = "explore",
            },
            CompleteSamplingWire.CaseRuns),
        ["RunRuns of no behavior"] = (
            new Wire.RunRunsRequest { ModelHash = Hash, Runs = -2 },
            CompleteSamplingWire.Unconcluded),
        ["SelfCheck"] = (
            new Wire.SelfCheckRequest { ModelHash = Hash, SelfCheckPackage = { "Rules", "More::Rules" } },
            CompleteSamplingWire.SelfCheck),
        ["RecordRun"] = (
            new Wire.RecordRunRequest
            {
                ModelHash = Hash,
                AnalysisSymbolId = "Sw::CostAnalysis",
                SubjectSymbolId = "Sw::barge",
                Arguments = { SendableValues.Values },
                NamedArguments = { SendableValues },
                Engine = "sweep",
                RecordInto = "Sw::Records",
                Ranges =
                {
                    new Wire.SweepRange { Parameter = "limit", Start = new Wire.Value { RealValue = 0 }, End = new Wire.Value { RealValue = 40 }, Step = new Wire.Value { RealValue = 20 } },
                    new Wire.SweepRange { Start = new Wire.Value { IntValue = -1 }, End = new Wire.Value { Quantity = new Wire.Quantity { IntMagnitude = 1 } } },
                },
                Runs = 3,
                Seed = ulong.MaxValue,
                AcceptDocuments = true,
            },
            CompleteSamplingWire.SweepRecording),
        ["RecordRun once"] = (
            new Wire.RecordRunRequest { ModelHash = Hash, AnalysisSymbolId = "Sw::CostAnalysis", SubjectSymbolId = "Sw::barge", AcceptDocuments = true },
            CompleteSamplingWire.Recording),
        ["RecordRun of a sample"] = (
            new Wire.RecordRunRequest { ModelHash = Hash, AnalysisSymbolId = "MC::Mc", Runs = 2, Seed = 0, AcceptDocuments = true },
            CompleteSamplingWire.SampleRecording),
        ["Convert"] = (
            new Wire.ConvertRequest
            {
                Content = "package P;",
                FromFormat = "kerml",
                ToFormat = "api-json",
                TolerateSyntaxErrors = true,
                IdForm = "uuid",
                Compact = true,
                OmitDerived = true,
                KeepDerived = { "owner" },
                IncludeCheckDiagnostics = true,
                IncludeLibraryStubs = true,
            },
            CompleteConversionWire.ApiJsonConversion),
        ["Convert from a file"] = (
            new Wire.ConvertRequest { FilePath = "/models/a.ttl", ToFormat = "sysml", IdForm = "qualified" },
            new Wire.ConvertResponse { Content = "package P;\n", FromFormat = "ttl", ToFormat = "sysml", Experimental = true, ExperimentalNotice = "n" }),
        ["Convert a model"] = (
            new Wire.ConvertRequest { ModelHash = Hash, ToFormat = "ttl", Documents = { "a.sysml" }, FromFormat = "sysml" },
            CompleteConversionWire.Conversion),
        ["Migrate"] = (
            new Wire.MigrateRequest
            {
                Content = ByteString.CopyFrom(1, 2, 3),
                FromFormat = "mdzip",
                ToFormat = "sysml",
                Report = true,
                Results = true,
                LayoutContent = "<mtip/>",
                ImageBaseUrl = "https://example.com",
                Strict = true,
            },
            CompleteConversionWire.Migration),
        ["Query"] = (
            new Wire.QueryRequest
            {
                ModelHash = Hash,
                Query = new Wire.Query
                {
                    Scope = { "Demo::vehicle" },
                    Select = { "name", "owner" },
                    Where = new Wire.Constraint
                    {
                        Composite = new Wire.CompositeConstraint
                        {
                            Operator = Wire.CompositeOperator.Or,
                            Constraint =
                            {
                                new Wire.Constraint { Primitive = new Wire.PrimitiveConstraint { Property = "@type", Operator = Wire.PrimitiveOperator.Equal, Value = { "PartUsage", "PartDefinition" } } },
                                new Wire.Constraint { Primitive = new Wire.PrimitiveConstraint { Inverse = true, Property = "multiplicityUpper", Operator = Wire.PrimitiveOperator.Less, Value = { "4" } } },
                                new Wire.Constraint
                                {
                                    Composite = new Wire.CompositeConstraint
                                    {
                                        Operator = Wire.CompositeOperator.And,
                                        Constraint =
                                        {
                                            new Wire.Constraint { Primitive = new Wire.PrimitiveConstraint { Property = "multiplicityLower", Operator = Wire.PrimitiveOperator.Greater, Value = { "0" } } },
                                            new Wire.Constraint { Primitive = new Wire.PrimitiveConstraint { Property = "multiplicityLower", Operator = Wire.PrimitiveOperator.GreaterEqual, Value = { "1" } } },
                                            new Wire.Constraint { Primitive = new Wire.PrimitiveConstraint { Property = "multiplicityUpper", Operator = Wire.PrimitiveOperator.LessEqual, Value = { "8" } } },
                                            new Wire.Constraint { Primitive = new Wire.PrimitiveConstraint { Property = "name", Operator = Wire.PrimitiveOperator.In, Value = { "wheels", "spare" } } },
                                            new Wire.Constraint { Primitive = new Wire.PrimitiveConstraint { Inverse = true, Property = "@type", Operator = Wire.PrimitiveOperator.InstanceOf, Value = { "Definition" } } },
                                        },
                                    },
                                },
                            },
                        },
                    },
                    OrderBy = { "name", "qualifiedName" },
                },
            },
            CompleteQueryDocsWire.Query),
        ["Query of the whole model"] = (new Wire.QueryRequest { ModelHash = Hash, Query = new Wire.Query() }, new Wire.QueryResponse()),
        ["Query by OSLC text"] = (new Wire.QueryRequest { ModelHash = Hash, OslcQuery = "oslc.where=rdf:type=\"PartUsage\"" }, CompleteQueryDocsWire.Query),
        ["RunDocumentQuery"] = (
            new Wire.RunDocumentQueryRequest
            {
                ModelHash = Hash,
                QueryId = "Garage::Checks",
                Bindings =
                {
                    new Wire.DocumentQueryBinding
                    {
                        Parameter = "root",
                        Values =
                        {
                            new Wire.DocumentValue { ElementId = "Garage::car" },
                            new Wire.DocumentValue { ElementId = "Garage::Car", ElementType = "PartDefinition" },
                            new Wire.DocumentValue { Object = new Wire.DocumentObject { Path = "Garage::car.wheels[2]" } },
                            new Wire.DocumentValue { Object = CompleteExecutionWire.Object },
                            new Wire.DocumentValue { Object = new Wire.DocumentObject { InstanceId = 3, Element = new Wire.DocumentValue { ElementType = "PartUsage" } } },
                        },
                    },
                    new Wire.DocumentQueryBinding
                    {
                        Parameter = "threshold",
                        Values =
                        {
                            new Wire.DocumentValue { StringValue = "10" },
                            new Wire.DocumentValue { IntValue = -3 },
                            new Wire.DocumentValue { BigIntValue = "-9223372036854775809" },
                            new Wire.DocumentValue { RationalValue = new Wire.Rational { Numerator = "1", Denominator = "3" } },
                            new Wire.DocumentValue { RealValue = 2.5 },
                            new Wire.DocumentValue { BoolValue = true },
                            new Wire.DocumentValue
                            {
                                Quantity = new Wire.Quantity
                                {
                                    RealMagnitude = 2.5,
                                    Unit = "km",
                                    UnitTerm = new Wire.UnitTerm { ScaleNum = 1000, ScaleDen = 1, Factors = { new Wire.UnitFactor { UnitId = "ISQBase::L", Exponent = 1 } } },
                                },
                            },
                        },
                    },
                    new Wire.DocumentQueryBinding { Parameter = "root", Values = { new Wire.DocumentValue { ElementId = "Garage::bike" } } },
                    new Wire.DocumentQueryBinding { Parameter = "none" },
                },
            },
            CompleteQueryDocsWire.DocumentQuery),
        ["RenderDocument"] = (new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = "Observatory::MassReport" }, CompleteQueryDocsWire.Markdown),
        ["RenderDocument as HTML"] = (new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = "Observatory::MassReport", Form = "html" }, CompleteQueryDocsWire.Html),
        ["RenderView"] = (
            new Wire.RenderViewRequest { ModelHash = Hash, View = "PlantViews::placedView", Ports = "full", Show = { "all", "typings" }, Depth = 3 },
            CompleteQueryDocsWire.View),
        ["RenderView of a pseudo-view"] = (new Wire.RenderViewRequest { ModelHash = Hash, View = "#tree:Plant::Loop" }, CompleteQueryDocsWire.SparseView),
        ["Migrate from a file"] = (
            new Wire.MigrateRequest { FilePath = Path.GetFullPath("/models/a.xmi"), ToFormat = "ttl", LayoutPath = "/models/a_mtip.xml" },
            CompleteConversionWire.Migration),
        ["ApplyEdits"] = (CompleteEditingWire.Request, CompleteEditingWire.Single),
        ["ApplyEdits of a model of several documents"] = (
            new Wire.ApplyEditsRequest
            {
                ModelHash = Hash,
                AcceptDocuments = true,
                Operations = { new Wire.EditOperation { Rename = new Wire.RenameEdit { Target = "EngineLibrary::Engine", NewName = "Motor" } } },
            },
            CompleteEditingWire.Several),
        ["ExportGraphs"] = (CompleteGraphDataWire.ExportRequest, CompleteGraphDataWire.Graphs),
        ["ImportData"] = (CompleteGraphDataWire.ImportRequest, CompleteGraphDataWire.Imported),
        ["ImportData as a dry run"] = (CompleteGraphDataWire.DryRunRequest, CompleteGraphDataWire.DryRun),
        ["ImportData of a model of several documents"] = (CompleteGraphDataWire.ImportRequest, CompleteGraphDataWire.ImportedSeveral),
        ["ImportData declining several documents"] = (
            new Wire.ImportDataRequest { ModelHash = Hash, Imports = { new Wire.DataImport { Name = "fleet.csv", Content = "element\n" } } },
            new Wire.ImportDataResponse { Files = { new Wire.ImportedFile { Name = "fleet.csv" } }, ModelHash = Hash }),
        ["CompareResults"] = (CompleteGraphDataWire.CompareRequest, CompleteGraphDataWire.Comparison),
    };

    // Every value a request can send, one of each arm the API writes as given, by name.
    private static Dictionary<string, Wire.Value> SendableValues => CompleteWire.Values
        .Where(value => value.KindCase is not (Wire.Value.KindOneofCase.Unset or Wire.Value.KindOneofCase.Undetermined or Wire.Value.KindOneofCase.Null)
            && value.Function?.SelfId is null or 0)
        .Append(new Wire.Value { Null = string.Empty })
        .Append(new Wire.Value { Function = new Wire.Function { CalcId = "F::Sq" } })
        .Select((value, i) => (Name: $"v{i:D2}", Value: value))
        .ToDictionary(entry => entry.Name, entry => entry.Value, StringComparer.Ordinal);

    private static Dictionary<string, (IMessage Request, IMessage Response)> InBand => new(StringComparer.Ordinal)
    {
        ["ParseFile"] = (
            new Wire.ParseFileRequest { Content = "package P;" },
            new Wire.ParseFileResponse { Error = "the source could not be read", Diagnostics = { CompleteWire.Diagnostic } }),
        ["ParseSources"] = (
            new Wire.ParseSourcesRequest { Documents = { new Wire.SourceDocument { Content = "package A;", Name = "a.sysml" } } },
            new Wire.ParseSourcesResponse { Error = "the sources could not be read", Diagnostics = { CompleteWire.Diagnostic } }),
        ["GetSymbol"] = (
            new Wire.GetSymbolRequest { ModelHash = Hash, SymbolId = "Demo::Nope" },
            new Wire.SymbolResponse { Error = "symbol not found: Demo::Nope" }),
        ["GetDiagnostics"] = (
            new Wire.DiagnosticsRequest { ModelHash = Hash },
            new Wire.DiagnosticsResponse { Error = "the lints did not run", Diagnostics = { CompleteWire.Diagnostic } }),
        ["Evaluate"] = (
            new Wire.EvaluateRequest { ModelHash = Hash, Expression = "invalid syntax (((" },
            new Wire.EvaluateResponse { Error = "unexpected trailing text", Diagnostics = { CompleteWire.Diagnostic } }),
        ["Instantiate"] = (
            new Wire.InstantiateRequest { ModelHash = Hash, SymbolId = "Test::DoesNotExist" },
            new Wire.InstantiateResponse { Error = "symbol not found: Test::DoesNotExist", Diagnostics = { CompleteWire.Diagnostic } }),
        ["ExecuteAction"] = (
            new Wire.ExecuteActionRequest { ModelHash = Hash, ActionSymbolId = "Timed::delayed", Advance = 2 },
            CompleteExecutionWire.FailedAction),
        ["ExecuteState"] = (
            new Wire.ExecuteStateRequest { ModelHash = Hash, StateMachineSymbolId = "Test::Machine", Trace = true },
            CompleteExecutionWire.FailedState),
        ["CheckBehavior"] = (
            new Wire.CheckBehaviorRequest { ModelHash = Hash, ActionSymbolId = "Plant::Tank::drain" },
            CompleteExecutionWire.FailedCheck),
        ["VerifyConstraint"] = (
            new Wire.VerifyConstraintRequest { ModelHash = Hash, SymbolId = "Demo::nope" },
            new Wire.VerifyConstraintResponse { Error = "symbol not found: Demo::nope", Instances = { CompleteWire.Instance }, Diagnostics = { CompleteWire.Diagnostic } }),
        ["VerifyRequirement"] = (
            new Wire.VerifyRequirementRequest { ModelHash = Hash, SymbolId = "Demo::nope" },
            new Wire.VerifyRequirementResponse
            {
                Error = "symbol not found: Demo::nope",
                Instances = { CompleteWire.Instance },
                Diagnostics = { CompleteWire.Diagnostic },
                VerificationVerdicts = { CompleteVerificationWire.Verification },
            }),
        ["VerifySatisfaction"] = (
            new Wire.VerifySatisfactionRequest { ModelHash = Hash, SymbolId = "Demo" },
            new Wire.VerifySatisfactionResponse
            {
                Error = "Demo states no satisfaction assertion",
                FailureReason = Wire.FailureReason.WrongKind,
                Instances = { CompleteWire.Instance },
                Diagnostics = { CompleteWire.Diagnostic },
                VerificationVerdicts = { CompleteVerificationWire.Verification },
            }),
        ["ValidateInstance"] = (
            new Wire.ValidateInstanceRequest { ModelHash = Hash, SymbolId = "Demo" },
            new Wire.ValidateInstanceResponse
            {
                Error = "Demo declares no object",
                FailureReason = Wire.FailureReason.WrongKind,
                Instances = { CompleteWire.Instance },
                Diagnostics = { CompleteWire.Diagnostic },
                VerificationVerdicts = { CompleteVerificationWire.Verification },
            }),
        ["EvaluateCalc"] = (
            new Wire.EvaluateCalcRequest { ModelHash = Hash, SymbolId = "Demo::sedan" },
            new Wire.EvaluateCalcResponse
            {
                Error = "Demo::sedan declares no calc",
                FailureReason = Wire.FailureReason.WrongKind,
                Diagnostics = { CompleteWire.Diagnostic },
                Engine = "run",
                Strength = "not covered",
                Bounds = { CompleteVerificationWire.Bounds },
            }),
        ["RunAnalysis"] = (
            new Wire.RunAnalysisRequest { ModelHash = Hash, SymbolId = "An::CostAnalysis" },
            CompleteVerificationWire.FailedAnalysis),
        ["RunSweep"] = (
            new Wire.RunSweepRequest { ModelHash = Hash, SymbolId = "Sw::barge" },
            new Wire.RunSweepResponse
            {
                Error = "Sw::barge declares neither an analysis case nor a calc",
                FailureReason = Wire.FailureReason.Evaluation,
                Instances = { CompleteWire.Instance },
                Diagnostics = { CompleteWire.Diagnostic },
                Engine = "sweep",
                Strength = "observed",
                Bounds = { CompleteVerificationWire.Bounds },
            }),
        ["RunRuns"] = (
            new Wire.RunRunsRequest { ModelHash = Hash, ActionSymbolId = "MC::Settle", Runs = 1, Seed = 7, Observe = { "nope" } },
            CompleteSamplingWire.Unobserved),
        ["SelfCheck"] = (
            new Wire.SelfCheckRequest { ModelHash = Hash, SelfCheckPackage = { "Nope" } },
            new Wire.SelfCheckResponse { Error = "-self-check-package Nope names no package", Diagnostics = { CompleteWire.Diagnostic } }),
        ["RecordRun"] = (
            new Wire.RecordRunRequest { ModelHash = Hash, AnalysisSymbolId = "Sw::CostAnalysis", AcceptDocuments = true },
            CompleteSamplingWire.FailedRunRecording),
        ["Convert"] = (
            new Wire.ConvertRequest { Content = "package P {", FromFormat = "sysml", ToFormat = "api-json" },
            CompleteConversionWire.FailedConversion),
        ["Migrate"] = (
            new Wire.MigrateRequest { Content = ByteString.CopyFromUtf8("not xmi"), FromFormat = "xmi", ToFormat = "ttl" },
            CompleteConversionWire.FailedMigration),
        ["ApplyEdits"] = (
            new Wire.ApplyEditsRequest
            {
                ModelHash = Hash,
                AcceptDocuments = true,
                Operations = { new Wire.EditOperation { Delete = new Wire.DeleteEdit { Target = "EngineLibrary::Engine" } } },
            },
            CompleteEditingWire.Refusal),
        ["ImportData"] = (CompleteGraphDataWire.ImportRequest, CompleteGraphDataWire.Refusal),
        ["CompareResults"] = (CompleteGraphDataWire.CompareRequest, CompleteGraphDataWire.CompareRefusal),
    };

    [Theory]
    [MemberData(nameof(Exchanges))]
    public async Task EachRpcSendsTheScenarioRequestThroughItsTypedCallAndRendersTheWholeAnswer(ClientWire wire, string name)
    {
        var (request, response) = Complete[name];
        var method = ScenarioRequests.Method(name.Split(' ')[0]);
        using var handler = Serving(wire, method.Name, response);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var answer = await Client(connection).CallAsync(method, request, TestContext.Current.CancellationToken);

        Assert.Equal(new ScenarioAnswer.Answered(response), answer);
        var sent = Assert.Single(handler.RequestsFor(method.Name));
        Assert.Equal(request, Decoded(sent, method.InputType));
    }

    [Theory]
    [MemberData(nameof(Failures))]
    public async Task AFailureTheAnswerReportsIsRenderedInBand(ClientWire wire, string name)
    {
        var (request, response) = InBand[name];
        var method = ScenarioRequests.Method(name);
        using var handler = Serving(wire, method.Name, response);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var answer = await Client(connection).CallAsync(method, request, TestContext.Current.CancellationToken);

        Assert.Equal(new ScenarioAnswer.Answered(response), answer);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARefusalKeepsTheStatusAndMessageTheServiceGave(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Refusing("Evaluate", StatusCode.NotFound, "model not found: h")
            .Refusing("GetDiagnostics", StatusCode.InvalidArgument, "unknown lint code no-such-lint")
            .Refusing("ParseFile", StatusCode.NotFound, "open /nonexistent: no such file");
        await using var connection = await FakeConnection.OpenAsync(handler);
        var client = Client(connection);
        var token = TestContext.Current.CancellationToken;

        var evaluate = await client.CallAsync(ScenarioRequests.Method("Evaluate"), new Wire.EvaluateRequest { ModelHash = Hash, Expression = "1" }, token);
        var lint = await client.CallAsync(ScenarioRequests.Method("GetDiagnostics"), new Wire.DiagnosticsRequest { ModelHash = Hash, DisableLint = { "no-such-lint" } }, token);
        var file = await client.CallAsync(ScenarioRequests.Method("ParseFile"), new Wire.ParseFileRequest { FilePath = "/nonexistent" }, token);

        Assert.Equal(new ScenarioAnswer.Refused(StatusCode.NotFound, "model not found: h"), evaluate);
        Assert.Equal(new ScenarioAnswer.Refused(StatusCode.InvalidArgument, "unknown lint code no-such-lint"), lint);
        Assert.Equal(new ScenarioAnswer.Refused(StatusCode.NotFound, "open /nonexistent: no such file"), file);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AFaultInTheModelsDocumentDeclarationsIsRenderedAsTheRefusalTheServiceMade(ClientWire wire)
    {
        const string Broken = "unknown property \"nope\" (declared in document.sysml)";
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Refusing("RunDocumentQuery", StatusCode.FailedPrecondition, Broken)
            .Refusing("RenderDocument", StatusCode.FailedPrecondition, Broken);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var client = Client(connection);
        var token = TestContext.Current.CancellationToken;

        var query = await client.CallAsync(ScenarioRequests.Method("RunDocumentQuery"), new Wire.RunDocumentQueryRequest { ModelHash = Hash, QueryId = "Q" }, token);
        var document = await client.CallAsync(ScenarioRequests.Method("RenderDocument"), new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = "D" }, token);

        Assert.Equal(new ScenarioAnswer.Refused(StatusCode.FailedPrecondition, Broken), query);
        Assert.Equal(new ScenarioAnswer.Refused(StatusCode.FailedPrecondition, Broken), document);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ACapabilityTheServiceLacksIsRefusedAsUnimplementedBeforeAnythingIsSent(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = [] };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var client = Client(connection);
        var token = TestContext.Current.CancellationToken;

        var strict = await client.CallAsync(ScenarioRequests.Method("ParseFile"), new Wire.ParseFileRequest { Content = "package P;", StrictConformance = true }, token);
        var subject = await client.CallAsync(
            ScenarioRequests.Method("Evaluate"), new Wire.EvaluateRequest { ModelHash = Hash, Expression = "mass", SubjectSymbolId = "Demo::sedan" }, token);
        var sources = await client.CallAsync(
            ScenarioRequests.Method("ParseSources"), new Wire.ParseSourcesRequest { Documents = { new Wire.SourceDocument { Content = "package A;" } } }, token);

        Assert.All([strict, subject, sources], answer => Assert.Equal(StatusCode.Unimplemented, Assert.IsType<ScenarioAnswer.Refused>(answer).Status));
        Assert.Contains(Capabilities.StrictConformance, ((ScenarioAnswer.Refused)strict).Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnAnswerOfNeitherResultNorFailureIsTheTransportsInternalStatus(ClientWire wire)
    {
        using var handler = Serving(wire, "Evaluate", new Wire.EvaluateResponse());
        await using var connection = await FakeConnection.OpenAsync(handler);

        var answer = await Client(connection).CallAsync(
            ScenarioRequests.Method("Evaluate"), new Wire.EvaluateRequest { ModelHash = Hash, Expression = "1" }, TestContext.Current.CancellationToken);

        Assert.Equal(StatusCode.Internal, Assert.IsType<ScenarioAnswer.Refused>(answer).Status);
    }

    [Theory]
    [MemberData(nameof(UnexpressibleRequests))]
    public async Task ARequestTheApiCannotExpressIsSkippedForItsReasonAndSendsNothing(string rpc, string json, string reason)
    {
        var method = ScenarioRequests.Method(rpc);
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var answer = await Client(connection).CallAsync(method, JsonParser.Default.Parse(json, method.InputType), TestContext.Current.CancellationToken);

        Assert.Equal(new ScenarioAnswer.Unexpressible(reason), answer);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task InvalidOrNoncanonicalMigrationFilePathsAreUnexpressibleWithoutChangingTheRequest(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);
        foreach (var (path, reason) in new[]
        {
            (Path.Combine(Path.GetTempPath(), "a\0b.xmi"), "the public API requires a fully-qualified, valid migration file path"),
            (Path.Combine(Path.GetTempPath(), "models", "..", "Vehicle.xmi"), "the public API normalizes a migration file path before sending it"),
        })
        {
            var request = new Wire.MigrateRequest { FilePath = path, ToFormat = "sysml" };
            var answer = await Client(connection).CallAsync(ScenarioRequests.Method("Migrate"), request, TestContext.Current.CancellationToken);
            Assert.Equal(new ScenarioAnswer.Unexpressible(reason), answer);
        }

        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Fact]
    public async Task AValueThePublicTypesRefuseAsSpelledIsUnexpressibleForTheirReason()
    {
        var method = ScenarioRequests.Method("ExecuteAction");
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var request = JsonParser.Default.Parse("""{"model_hash": "h", "inputs": {"x": {"array": {"dimensions": [3], "elements": [{"int_value": 1}]}}}}""", method.InputType);

        var answer = await Client(connection).CallAsync(method, request, TestContext.Current.CancellationToken);

        var reason = Assert.IsType<ScenarioAnswer.Unexpressible>(answer).Reason;
        Assert.StartsWith("the public API cannot hold inputs[\"x\"] as spelled: An array", reason, StringComparison.Ordinal);
        Assert.DoesNotContain("(Parameter", reason, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(sent => sent.Method));
    }

    [Fact]
    public async Task AnActionBodyNestedDeeperThanTheServiceReadsIsUnexpressible()
    {
        var method = ScenarioRequests.Method("ApplyEdits");
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var item = new Wire.AddSequenceEdit { Keyword = "then", Ref = "x" };
        for (var level = 0; level <= Edit.AddSequence.MaxDepth; level++)
        {
            item = new Wire.AddSequenceEdit { MemberKind = "loop", Body = { item } };
        }

        var request = new Wire.ApplyEditsRequest { ModelHash = Hash, Operations = { new Wire.EditOperation { AddSequence = item } } };
        var answer = await Client(connection).CallAsync(method, request, TestContext.Current.CancellationToken);

        var reason = Assert.IsType<ScenarioAnswer.Unexpressible>(answer).Reason;
        Assert.StartsWith("the public API cannot hold operations[0] as spelled: Item 0 spans 129 levels", reason, StringComparison.Ordinal);
        Assert.DoesNotContain("(Parameter", reason, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(sent => sent.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ServerInfoIsWhatTheConnectionKeptWhenItOpened(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Version = "v1.2.3", Capabilities = ["query", "convert", "apply_edits"] };
        using var silent = new FakeServiceHandler(wire)
        {
            ServerInfo = (_, _) => Task.FromResult(FakeServiceHandler.RefuseOn(wire, StatusCode.Unimplemented, "unknown method")),
        };
        await using var connection = await FakeConnection.OpenAsync(handler);
        await using var old = await FakeConnection.OpenAsync(silent);
        var method = ScenarioRequests.Method("GetServerInfo");
        var token = TestContext.Current.CancellationToken;

        var answer = await Client(connection).CallAsync(method, new Wire.ServerInfoRequest(), token);
        var refused = await Client(old).CallAsync(method, new Wire.ServerInfoRequest(), token);

        Assert.Equal(
            new ScenarioAnswer.Answered(new Wire.ServerInfoResponse { Version = "v1.2.3", Capabilities = { "apply_edits", "convert", "query" } }),
            answer);
        Assert.Equal(StatusCode.Unimplemented, Assert.IsType<ScenarioAnswer.Refused>(refused).Status);
        Assert.Single(handler.RequestsFor("GetServerInfo"));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AScenarioModelIsParsedAsOneFileOrAsNamedDocuments(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .On("ParseFile", Wire.ParseFileRequest.Parser, _ => new Wire.ParseFileResponse { ModelHash = "one", Diagnostics = { CompleteWire.Diagnostic } })
            .On("ParseSources", Wire.ParseSourcesRequest.Parser, _ => new Wire.ParseSourcesResponse { Error = "cannot", Diagnostics = { CompleteWire.BareDiagnostic } });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var client = Client(connection);
        var token = TestContext.Current.CancellationToken;

        var single = await client.ParseAsync(new ScenarioModel("a.sysml", [], "kerml", true), [new FixtureSource("a.sysml", "package A;")], token);
        var several = await client.ParseAsync(
            new ScenarioModel("", ["a.sysml", "b.sysml"], "", false), [new FixtureSource("a.sysml", "package A;"), new FixtureSource("b.sysml", "package B;")], token);

        Assert.Equal(("one", ""), (single.Hash, single.Error));
        Assert.Equal([new ModelParseDiagnostic("warning", "rounded")], single.Diagnostics);
        Assert.Equal(("", "cannot"), (several.Hash, several.Error));
        Assert.Equal([new ModelParseDiagnostic("hint", "consider")], several.Diagnostics);
        Assert.Equal(
            new Wire.ParseFileRequest { Content = "package A;", Language = "kerml", StrictConformance = true },
            Assert.Single(handler.RequestsFor("ParseFile")).Message(Wire.ParseFileRequest.Parser));
        Assert.Equal(
            new Wire.ParseSourcesRequest
            {
                Documents = { new Wire.SourceDocument { Content = "package A;", Name = "a.sysml" }, new Wire.SourceDocument { Content = "package B;", Name = "b.sysml" } },
            },
            Assert.Single(handler.RequestsFor("ParseSources")).Message(Wire.ParseSourcesRequest.Parser));
    }

    [Fact]
    public void EveryAdapterReadsItsRpcsRequestAndSendsOnlyFieldsTheInventoryTypes()
    {
        var inventory = ParityInventory.LoadCommitted();
        foreach (var (name, adapter) in ApiAdapters.All)
        {
            var method = ScenarioRequests.Method(name);
            var fields = inventory.Fields[method.InputType.FullName];

            Assert.Equal(method.InputType.ClrType, adapter.RequestType);
            Assert.Equal(
                fields.Where(field => field.Value.Status == "typed").Select(field => field.Key).Order(StringComparer.Ordinal),
                adapter.Fields.Order(StringComparer.Ordinal));
        }
    }

    [Fact]
    public void TheAdaptersCoverEveryImplementedRpcAndTheExpectedSkipsListEveryDeferredOne()
    {
        var rows = ParityInventory.LoadCommitted().Rpcs;
        var expected = ExpectedSkips.Load(Repository.PathOf("client", "dotnet", "test", "testdata", "expected-skips.json"));

        Assert.Equal(
            rows.Where(row => row.Status == "implemented").Select(row => row.Rpc).Order(StringComparer.Ordinal),
            ApiAdapters.All.Keys.Order(StringComparer.Ordinal));
        Assert.Equal(
            rows.Where(row => row.Status == "deferred").Select(row => row.Rpc).Order(StringComparer.Ordinal),
            expected.UncoveredRpcs.Order(StringComparer.Ordinal));
        Assert.Empty(expected.Problems(ScenarioSuite.Load(Repository.PathOf("conformance", "scenarios")), ScenarioRequests.Service, ApiAdapters.All.Keys));
    }

    [Fact]
    public void NothingInTheRunnerReachesTheServiceRaw()
    {
        var raw = new HashSet<MethodBase>(typeof(Connection).GetMethods().Where(method => method.Name == nameof(Connection.CallAsync)));
        var stub = typeof(Wire.SysMLService.SysMLServiceClient);
        var calls = new List<string>();
        foreach (var type in typeof(ApiScenarioClient).Assembly.GetTypes())
        {
            foreach (var method in type.GetMethods(BindingFlags.Instance | BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.DeclaredOnly)
                .Cast<MethodBase>()
                .Concat(type.GetConstructors(BindingFlags.Instance | BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic)))
            {
                foreach (var called in Called(method))
                {
                    var definition = called is MethodInfo { IsGenericMethod: true } generic ? generic.GetGenericMethodDefinition() : called;
                    if (raw.Contains(definition) || called.DeclaringType == stub)
                    {
                        calls.Add($"{type.FullName}.{method.Name} calls {called.DeclaringType}.{called.Name}");
                    }
                }
            }
        }

        Assert.Empty(calls);
        // The scan is not blind: it finds the typed call an adapter does make.
        var evaluate = typeof(ModelArea).GetMethod("EvaluateAsync", BindingFlags.Static | BindingFlags.NonPublic)!
            .GetCustomAttribute<System.Runtime.CompilerServices.AsyncStateMachineAttribute>()!.StateMachineType
            .GetMethod("MoveNext", BindingFlags.Instance | BindingFlags.NonPublic)!;
        Assert.Contains(Called(evaluate), called => called.DeclaringType == typeof(Model) && called.Name == nameof(Model.EvaluateAsync));
    }

    private static ApiScenarioClient Client(Connection connection) => new("test", "fake", connection);

    private static FakeServiceHandler Serving(ClientWire wire, string method, IMessage response)
    {
        var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities };
        handler.Respond = (sent, _) => Task.FromResult(sent.Method == method ? handler.Answer(response) : handler.Refuse(StatusCode.Unimplemented, "not faked"));
        return handler;
    }

    private static IMessage Decoded(SentRequest sent, MessageDescriptor type) =>
        sent.Wire == ClientWire.ConnectJson
            ? JsonParser.Default.Parse(Encoding.UTF8.GetString(sent.Body), type)
            : type.Parser.ParseFrom(sent.MessageBytes);

    // The methods a method's IL calls, constructs or loads a pointer to.
    private static IEnumerable<MethodBase> Called(MethodBase method)
    {
        var il = method.GetMethodBody()?.GetILAsByteArray();
        if (il is null)
        {
            yield break;
        }

        for (var at = 0; at < il.Length;)
        {
            var code = il[at] == 0xFE ? Codes[0x100 | il[at + 1]] : Codes[il[at]];
            at += code.Size;
            var operand = Operand(code.OperandType, il, at);
            if (code.OperandType == OperandType.InlineMethod)
            {
                MethodBase? called = null;
                try
                {
                    called = method.Module.ResolveMethod(
                        BitConverter.ToInt32(il, at),
                        method.DeclaringType?.IsGenericType == true ? method.DeclaringType.GetGenericArguments() : null,
                        method.IsGenericMethod ? method.GetGenericArguments() : null);
                }
                catch (ArgumentException)
                {
                }

                if (called is not null)
                {
                    yield return called;
                }
            }

            at += operand;
        }
    }

    private static int Operand(OperandType type, byte[] il, int at) => type switch
    {
        OperandType.InlineNone => 0,
        OperandType.ShortInlineBrTarget or OperandType.ShortInlineI or OperandType.ShortInlineVar => 1,
        OperandType.InlineVar => 2,
        OperandType.InlineI8 or OperandType.InlineR => 8,
        OperandType.InlineSwitch => 4 + (4 * BitConverter.ToInt32(il, at)),
        _ => 4,
    };

    private static readonly Dictionary<int, OpCode> Codes = typeof(OpCodes).GetFields(BindingFlags.Public | BindingFlags.Static)
        .Select(field => (OpCode)field.GetValue(null)!)
        .ToDictionary(code => code.Size == 1 ? code.Value & 0xFF : 0x100 | (code.Value & 0xFF));
}
