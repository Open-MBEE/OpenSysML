using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.RegularExpressions;
using Google.Protobuf.Reflection;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Holds a <see cref="ParityInventory"/> to a schema, the conformance scenarios, the service's
/// capability names and the test suite, and reports every disagreement. It compares descriptor
/// sets, never counts: an RPC, field or arm the schema adds, renames or removes is a problem
/// until the inventory classifies it.
/// </summary>
internal static partial class ParityChecker
{
    /// <summary>The areas of the client an RPC or a set of arms is delivered by.</summary>
    internal static readonly IReadOnlySet<string> Areas = new HashSet<string>(StringComparer.Ordinal)
    {
        "connection", "model", "values", "execution", "verification", "sampling", "conversion", "query-docs", "editing", "graph-data",
    };

    private const string Always = "always";
    private const string Deferred = "deferred";
    private const string Implemented = "implemented";
    private const string Ungated = "ungated";
    private const string ResponseOnly = "response-only";

    private static readonly string[] FieldStatuses = [Deferred, "typed", "raw-only", "unsupported"];

    [GeneratedRegex(@"^(Connection|Model)\.([A-Z]\w*\(\)\.)?[A-Z]\w*Async$")]
    private static partial Regex PublicMethod();

    /// <summary>Every disagreement between the inventory and the rest, in a stable order.</summary>
    /// <param name="inventory">The inventory.</param>
    /// <param name="schema">The schema file the client was generated from.</param>
    /// <param name="scenarios">The conformance scenarios.</param>
    /// <param name="capabilities">The capability names the service declares.</param>
    /// <param name="tests">The test methods of the suite, as Class.Method.</param>
    /// <returns>The problems; none when the inventory is current.</returns>
    internal static IReadOnlyList<string> Check(
        ParityInventory inventory,
        FileDescriptor schema,
        IReadOnlyList<Scenario> scenarios,
        IReadOnlySet<string> capabilities,
        IReadOnlySet<string> tests)
    {
        var problems = new List<string>();
        var service = schema.Services.FirstOrDefault(service => service.FullName == inventory.Service);
        if (service is null)
        {
            problems.Add($"The schema declares no service {inventory.Service}.");
            return problems;
        }

        var context = new Context(schema, capabilities, tests, problems);
        var values = context.Message(inventory.Values.Message, "the values section");
        var edits = context.Message(inventory.Edits.Message, "the edits section");
        CheckRows(context, inventory, service, scenarios, values, edits);
        if (values is not null)
        {
            CheckValueArms(context, inventory.Values, values);
        }

        if (edits is not null)
        {
            CheckEditArms(context, inventory.Edits, edits);
        }

        CheckPopulation(context, inventory.Population);
        CheckFields(context, inventory.Fields, service);
        return problems;
    }

    private static void CheckRows(
        Context context,
        ParityInventory inventory,
        ServiceDescriptor service,
        IReadOnlyList<Scenario> scenarios,
        MessageDescriptor? values,
        MessageDescriptor? edits)
    {
        var rows = new Dictionary<string, RpcRow>(StringComparer.Ordinal);
        foreach (var row in inventory.Rpcs)
        {
            if (!rows.TryAdd(row.Rpc, row))
            {
                context.Problem($"RPC {row.Rpc} has more than one row.");
            }
        }

        var declared = service.Methods.Select(method => method.Name).ToList();
        foreach (var name in declared.Where(name => !rows.ContainsKey(name)))
        {
            context.Problem($"RPC {name} has no inventory row.");
        }

        foreach (var name in rows.Keys.Where(name => service.FindMethodByName(name) is null))
        {
            context.Problem($"Row {name} names no RPC of {service.FullName}.");
        }

        var listed = inventory.Rpcs.Select(row => row.Rpc).Where(declared.Contains).Distinct().ToList();
        if (!listed.SequenceEqual(declared.Where(rows.ContainsKey)))
        {
            context.Problem("The rows are not in the order the schema declares the RPCs.");
        }

        var byRpc = scenarios.ToLookup(scenario => scenario.Rpc, StringComparer.Ordinal);
        foreach (var scenario in scenarios.Where(scenario => service.FindMethodByName(scenario.Rpc) is null))
        {
            context.Problem($"Scenario {scenario.Id} calls {scenario.Rpc}, which is no RPC of {service.FullName}.");
        }

        foreach (var duplicate in scenarios.GroupBy(scenario => scenario.Id, StringComparer.Ordinal).Where(group => group.Count() > 1))
        {
            context.Problem($"Scenario ID {duplicate.Key} is used more than once.");
        }

        foreach (var row in rows.Values)
        {
            var method = service.FindMethodByName(row.Rpc);
            if (method is not null)
            {
                CheckRow(context, row, method, byRpc[row.Rpc].ToList(), inventory, values, edits);
            }
        }
    }

    private static void CheckRow(
        Context context,
        RpcRow row,
        MethodDescriptor method,
        IReadOnlyList<Scenario> scenarios,
        ParityInventory inventory,
        MessageDescriptor? values,
        MessageDescriptor? edits)
    {
        var at = $"Row {row.Rpc}";
        if (row.Request != method.InputType.FullName)
        {
            context.Problem($"{at} gives request {row.Request}; the schema's is {method.InputType.FullName}.");
        }

        if (row.Response != method.OutputType.FullName)
        {
            context.Problem($"{at} gives response {row.Response}; the schema's is {method.OutputType.FullName}.");
        }

        if (row.Methods.Count == 0)
        {
            context.Problem($"{at} names no public method.");
        }

        foreach (var name in row.Methods.Where(name => !PublicMethod().IsMatch(name)))
        {
            context.Problem($"{at} names public method \"{name}\", which is not Connection.XAsync, Model.XAsync or Model.X().YAsync.");
        }

        context.Text(row.Input, $"{at}'s input");
        context.Text(row.Result, $"{at}'s result");
        context.Area(row.Area, at);
        context.Implementation(row.Status, row.Tests ?? [], at);

        foreach (var gate in row.Gates)
        {
            context.Gate(gate, method.InputType, $"{at}'s gate {gate.Capability}");
        }

        var valuePaths = values is null ? [] : ValuePaths(method.InputType, values);
        Compare(context, valuePaths, row.ValueFields, $"{at}'s valueFields", "the request field", "carries Values but is not listed", "carries no Value");

        Compare(
            context,
            scenarios.Select(scenario => scenario.Id).ToList(),
            row.Scenarios,
            $"{at}'s scenarios",
            "scenario",
            "is not listed",
            "is no scenario of the RPC");

        var covered = new HashSet<string>(row.Gates.Select(gate => gate.Capability), StringComparer.Ordinal);
        var request = Reachable([method.InputType]);
        var response = Reachable([method.OutputType]);
        if (edits is not null && request.Contains(edits))
        {
            covered.UnionWith(inventory.Edits.Arms.Values.SelectMany(arm => arm.Gates).Select(gate => gate.Capability));
        }

        if (values is not null && valuePaths.Count > 0)
        {
            covered.UnionWith(inventory.Values.Arms.Values.Select(arm => arm.Send));
        }

        if (values is not null && response.Contains(values))
        {
            covered.UnionWith(inventory.Values.Arms.Values.Select(arm => arm.Receive).OfType<string>());
        }

        foreach (var (path, capability) in inventory.Population)
        {
            var dot = path.LastIndexOf('.');
            if (dot > 0 && response.Any(message => message.FullName == path[..dot]))
            {
                covered.Add(capability);
            }
        }

        foreach (var scenario in scenarios)
        {
            foreach (var capability in scenario.RequiredCapabilities.Where(capability => !covered.Contains(capability)))
            {
                context.Problem(
                    $"Scenario {scenario.Id} requires {capability}, which no gate of {at}, no Value or edit arm it carries, and no field its response reaches names.");
            }
        }
    }

    private static void CheckValueArms(Context context, ValueArms section, MessageDescriptor message)
    {
        context.Area(section.Area, "The values section");
        var oneof = context.Oneof(message, section.Oneof, "The values section");
        if (oneof is null)
        {
            return;
        }

        CompareArms(context, oneof, section.Arms.Keys, "The values section");
        var kinds = new HashSet<string>(StringComparer.Ordinal);
        foreach (var (name, arm) in section.Arms)
        {
            var at = $"Value arm {name}";
            context.Text(arm.Kind, $"{at}'s kind");
            if (!kinds.Add(arm.Kind))
            {
                context.Problem($"{at} repeats kind {arm.Kind}.");
            }

            context.Implementation(arm.Status, arm.Test is null ? [] : [arm.Test], at);
            if (arm.Send is not (Ungated or ResponseOnly))
            {
                context.Capability(arm.Send, $"{at}'s send gate");
            }

            if (arm.Receive is not null)
            {
                context.Capability(arm.Receive, $"{at}'s receive gate");
            }
        }

        var gated = section.Arms.Values.Select(arm => arm.Send).Where(send => send is not (Ungated or ResponseOnly)).Distinct().ToList();
        CompareOrder(context, gated, section.SendGateOrder, "The values section's sendGateOrder");
    }

    private static void CheckEditArms(Context context, EditArms section, MessageDescriptor message)
    {
        context.Area(section.Area, "The edits section");
        context.Text(section.Checked, "The edits section's checked");
        var oneof = context.Oneof(message, section.Oneof, "The edits section");
        if (oneof is null)
        {
            return;
        }

        CompareArms(context, oneof, section.Arms.Keys, "The edits section");
        var edits = new HashSet<string>(StringComparer.Ordinal);
        foreach (var (name, arm) in section.Arms)
        {
            var at = $"Edit arm {name}";
            context.Text(arm.Edit, $"{at}'s edit");
            if (!edits.Add(arm.Edit))
            {
                context.Problem($"{at} repeats edit {arm.Edit}.");
            }

            context.Implementation(arm.Status, arm.Test is null ? [] : [arm.Test], at);
            var field = oneof.Fields.FirstOrDefault(field => field.Name == name);
            foreach (var gate in arm.Gates)
            {
                context.Gate(gate, field?.FieldType == FieldType.Message ? field.MessageType : null, $"{at}'s gate {gate.Capability}");
            }
        }

        var gated = section.Arms.Values.SelectMany(arm => arm.Gates).Select(gate => gate.Capability).Distinct().ToList();
        CompareOrder(context, gated, section.GateOrder, "The edits section's gateOrder");
    }

    private static void CheckPopulation(Context context, IReadOnlyDictionary<string, string> population)
    {
        foreach (var (path, capability) in population)
        {
            var dot = path.LastIndexOf('.');
            var message = dot > 0 ? context.Messages.GetValueOrDefault(path[..dot]) : null;
            if (message?.FindFieldByName(path[(dot + 1)..]) is null)
            {
                context.Problem($"Population entry {path} names no field of the schema.");
            }

            context.Capability(capability, $"Population entry {path}");
        }
    }

    private static void CheckFields(
        Context context,
        IReadOnlyDictionary<string, IReadOnlyDictionary<string, FieldClass>> fields,
        ServiceDescriptor service)
    {
        var reachable = Reachable(service.Methods.SelectMany(method => new[] { method.InputType, method.OutputType }))
            .ToDictionary(message => message.FullName, StringComparer.Ordinal);
        foreach (var name in reachable.Keys.Where(name => !fields.ContainsKey(name)).Order(StringComparer.Ordinal))
        {
            context.Problem($"Message {name} has no field classification.");
        }

        foreach (var name in fields.Keys.Where(name => !reachable.ContainsKey(name)))
        {
            context.Problem($"Field classifications name message {name}, which no RPC reaches.");
        }

        foreach (var (name, classified) in fields)
        {
            if (!reachable.TryGetValue(name, out var message))
            {
                continue;
            }

            foreach (var field in message.Fields.InDeclarationOrder().Where(field => !classified.ContainsKey(field.Name)))
            {
                context.Problem($"Field {field.FullName} has no classification.");
            }

            foreach (var (field, classification) in classified)
            {
                var path = $"{name}.{field}";
                if (message.FindFieldByName(field) is null)
                {
                    context.Problem($"Field {path} is classified but not declared.");
                }

                context.Classification(classification, path);
            }
        }
    }

    /// <summary>Every message the schema declares, nested ones included, by full name.</summary>
    internal static Dictionary<string, MessageDescriptor> Messages(FileDescriptor schema)
    {
        var messages = new Dictionary<string, MessageDescriptor>(StringComparer.Ordinal);
        var pending = new Stack<MessageDescriptor>(schema.MessageTypes);
        while (pending.Count > 0)
        {
            var message = pending.Pop();
            messages.Add(message.FullName, message);
            foreach (var nested in message.NestedTypes)
            {
                pending.Push(nested);
            }
        }

        return messages;
    }

    /// <summary>Every message the given ones reach through their fields, themselves included; map entries are seen through.</summary>
    internal static HashSet<MessageDescriptor> Reachable(IEnumerable<MessageDescriptor> roots)
    {
        var seen = new HashSet<MessageDescriptor>();
        var pending = new Stack<MessageDescriptor>(roots);
        while (pending.Count > 0)
        {
            var message = pending.Pop();
            if (!seen.Add(message))
            {
                continue;
            }

            foreach (var field in message.Fields.InDeclarationOrder())
            {
                if (Target(field) is { } target)
                {
                    pending.Push(target);
                }
            }
        }

        return seen;
    }

    /// <summary>The dotted paths of the fields under a request that hold a Value, followed once per path without cycles.</summary>
    internal static List<string> ValuePaths(MessageDescriptor request, MessageDescriptor value)
    {
        var paths = new List<string>();
        Walk(request, string.Empty, [request]);
        return paths;

        void Walk(MessageDescriptor message, string prefix, HashSet<MessageDescriptor> path)
        {
            foreach (var field in message.Fields.InDeclarationOrder())
            {
                var target = Target(field);
                if (target is null)
                {
                    continue;
                }

                if (target == value)
                {
                    paths.Add(prefix + field.Name);
                }
                else if (!path.Contains(target))
                {
                    Walk(target, $"{prefix}{field.Name}.", [.. path, target]);
                }
            }
        }
    }

    // A field's message, or for a map field its value's message: what the field holds.
    private static MessageDescriptor? Target(FieldDescriptor field)
    {
        if (field.FieldType != FieldType.Message)
        {
            return null;
        }

        if (!field.IsMap)
        {
            return field.MessageType;
        }

        var value = field.MessageType.FindFieldByNumber(2);
        return value.FieldType == FieldType.Message ? value.MessageType : null;
    }

    private static void CompareArms(Context context, OneofDescriptor oneof, IEnumerable<string> listed, string at) =>
        Compare(context, oneof.Fields.Select(field => field.Name).ToList(), listed.ToList(), $"{at}'s arms", $"arm of {oneof.FullName}", "is not classified", "is not declared");

    private static void Compare(
        Context context,
        IReadOnlyList<string> expected,
        IReadOnlyList<string> listed,
        string at,
        string what,
        string missing,
        string stale)
    {
        foreach (var name in expected.Where(name => !listed.Contains(name, StringComparer.Ordinal)))
        {
            context.Problem($"{at}: {what} {name} {missing}.");
        }

        foreach (var name in listed.Where(name => !expected.Contains(name, StringComparer.Ordinal)))
        {
            context.Problem($"{at}: {what} {name} {stale}.");
        }

        foreach (var name in listed.GroupBy(name => name, StringComparer.Ordinal).Where(group => group.Count() > 1))
        {
            context.Problem($"{at}: {what} {name.Key} is listed more than once.");
        }
    }

    private static void CompareOrder(Context context, IReadOnlyList<string> used, IReadOnlyList<string> order, string at) =>
        Compare(context, used, order, at, "capability", "is used but not ordered", "is ordered but no arm uses it");

    private sealed class Context(FileDescriptor schema, IReadOnlySet<string> capabilities, IReadOnlySet<string> tests, List<string> problems)
    {
        internal Dictionary<string, MessageDescriptor> Messages { get; } = Messages(schema);

        internal void Problem(string problem) => problems.Add(problem);

        internal MessageDescriptor? Message(string name, string at)
        {
            var message = Messages.GetValueOrDefault(name);
            if (message is null)
            {
                Problem($"{at} names message {name}, which the schema does not declare.");
            }

            return message;
        }

        internal OneofDescriptor? Oneof(MessageDescriptor message, string name, string at)
        {
            var oneof = message.Oneofs.FirstOrDefault(oneof => oneof.Name == name);
            if (oneof is null)
            {
                Problem($"{at} names oneof {name}, which {message.FullName} does not declare.");
            }

            return oneof;
        }

        internal void Text(string text, string at)
        {
            if (string.IsNullOrWhiteSpace(text))
            {
                Problem($"{at} is empty.");
            }
        }

        internal void Area(string area, string at)
        {
            if (!Areas.Contains(area))
            {
                Problem($"{at} names area {area}, which is not one of {string.Join(", ", Areas.Order(StringComparer.Ordinal))}.");
            }
        }

        internal void Capability(string capability, string at)
        {
            if (!capabilities.Contains(capability))
            {
                Problem($"{at} names capability {capability}, which the service does not declare.");
            }
        }

        internal void Test(string test, string at)
        {
            if (!tests.Contains(test))
            {
                Problem($"{at} names test {test}, which the suite does not have.");
            }
        }

        internal void Implementation(string status, IReadOnlyList<string> proofs, string at)
        {
            switch (status)
            {
                case Deferred when proofs.Count > 0:
                    Problem($"{at} is deferred but names tests.");
                    break;
                case Deferred:
                    break;
                case Implemented when proofs.Count == 0:
                    Problem($"{at} is implemented but names no test.");
                    break;
                case Implemented:
                    foreach (var proof in proofs)
                    {
                        Test(proof, at);
                    }

                    break;
                default:
                    Problem($"{at} has status {status}, which is neither {Deferred} nor {Implemented}.");
                    break;
            }
        }

        internal void Gate(Gate gate, MessageDescriptor? message, string at)
        {
            Capability(gate.Capability, at);
            Text(gate.When, $"{at}'s predicate");
            if (gate.Field is null)
            {
                if (gate.When != Always)
                {
                    Problem($"{at} is conditional but names no field.");
                }

                return;
            }

            if (gate.When == Always)
            {
                Problem($"{at} names field {gate.Field} but applies always.");
            }

            if (message is null || !Resolves(message, gate.Field))
            {
                Problem($"{at} names field {gate.Field}, which {message?.FullName ?? "its message"} does not declare.");
            }
        }

        internal void Classification(FieldClass classification, string path)
        {
            var at = $"Field {path}";
            if (!FieldStatuses.Contains(classification.Status, StringComparer.Ordinal))
            {
                Problem($"{at} has status {classification.Status}, which is not one of {string.Join(", ", FieldStatuses)}.");
                return;
            }

            if (classification.Status == Deferred)
            {
                if (classification.Reason is not null || classification.Test is not null)
                {
                    Problem($"{at} is deferred but gives a reason or a test.");
                }

                return;
            }

            if (classification.Test is null)
            {
                Problem($"{at} is {classification.Status} but names no test.");
            }
            else
            {
                Test(classification.Test, at);
            }

            if (classification.Status != "typed" && string.IsNullOrWhiteSpace(classification.Reason))
            {
                Problem($"{at} is {classification.Status} but gives no reason.");
            }
        }

        private static bool Resolves(MessageDescriptor message, string path)
        {
            var segments = path.Split('.');
            for (var i = 0; i < segments.Length; i++)
            {
                var field = message.FindFieldByName(segments[i]);
                if (field is null)
                {
                    return false;
                }

                if (i < segments.Length - 1)
                {
                    if (Target(field) is not { } next)
                    {
                        return false;
                    }

                    message = next;
                }
            }

            return true;
        }
    }
}
