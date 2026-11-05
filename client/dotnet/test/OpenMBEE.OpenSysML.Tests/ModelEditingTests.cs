using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// ApplyEdits through the typed API, over every wire: every arm and option sent exactly as the schema writes it, the
/// editor's order, snapshot and single application, the nested bodies a statement holds, the gates judged over the
/// whole batch in the service's order before anything is sent, the notation of a model of one document and of several
/// told apart, a refusal kept with its kind, diagnostics and referrers, a malformed answer refused, and the edited
/// notation saved without ever leaving a partial file.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelEditingTests : IDisposable
{
    private const string Hash = CompleteEditingWire.Hash;

    private static readonly string[] SequenceParts = ["Ref", "MemberKind", "MemberName", "Type", "After", "Condition", "Value", "Target", "Via", "Until", "Multiplicity", "Parameter"];

    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-edit-").FullName;

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task EveryArmAndEveryOptionIsSentAsTheSchemaWritesIt(ClientWire wire)
    {
        using var handler = Serving(wire).On("ApplyEdits", Wire.ApplyEditsRequest.Parser, request => request.Operations.Count > 1 ? CompleteEditingWire.Single : CompleteEditingWire.Bare);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var editor = model.Edit();
        foreach (var edit in CompleteEditingWire.Edits)
        {
            editor.Add(edit);
        }

        await model.ApplyEditsAsync(CompleteEditingWire.Edits, CompleteEditingWire.Options, Token);
        await editor.ApplyAsync(CompleteEditingWire.Options, Token);
        await model.ApplyEditsAsync([new Edit.SetValue("Demo::x", "1")], cancellationToken: Token);

        var sent = handler.RequestsFor("ApplyEdits").Select(request => request.Message(Wire.ApplyEditsRequest.Parser)).ToList();
        Assert.Equal([CompleteEditingWire.Request, CompleteEditingWire.Request], sent.Take(2));
        Assert.Equal(
            new Wire.ApplyEditsRequest
            {
                ModelHash = Hash,
                AcceptDocuments = true,
                Operations = { new Wire.EditOperation { SetValue = new Wire.SetValueEdit { Target = "Demo::x", Value = "1" } } },
            },
            sent[2]);
    }

    [Fact]
    public void TheSampleBatchHoldsEveryArmOnceAndSetsEveryFieldARequestReaches()
    {
        var accepting = CompleteEditingWire.Request;
        accepting.AcceptDocuments = true;
        var arms = Wire.EditOperation.Descriptor.Oneofs.Single().Fields.Select(field => field.Name);

        Assert.Equal([], CompleteWire.ReachableFields(Wire.ApplyEditsRequest.Descriptor).Except(CompleteWire.SetFields([CompleteEditingWire.Request, accepting])).ToArray());
        Assert.Equal(arms, CompleteEditingWire.Request.Operations.Select(operation => Wire.EditOperation.Descriptor.FindFieldByNumber((int)operation.OperationCase).Name));
        Assert.Equal(arms, CompleteEditingWire.Edits.Select(edit => edit.Arm));
    }

    [Fact]
    public async Task TheEditorsShorthandsWriteEachFormInTheOrderTheyAreCalled()
    {
        using var handler = Serving(ClientWire.Grpc);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var then = new Editor.Body().AddAssign("x", "1").AddThen("done");
        var otherwise = new Editor.Body().AddSend("ping", "peer");

        var editor = model.Edit()
            .SetValue("A::x", "1")
            .Rename("A::B", "C")
            .Delete("A::Old")
            .Delete("A::Older", cascade: true)
            .Move("A::W", "A::P")
            .AddMember("A", "part", "engine")
            .AddMember("A", "part", "spare", m => m with { Type = "Engine", Multiplicity = "[0..1]" })
            .AddParameter("A::act", "in", "speed", "Real")
            .AddParameter("A::act", "out", "done")
            .AddConnection("A", "allocation", "a", "b")
            .AddConnection("A", "flow", "a", "b", c => c with { Name = "f" })
            .AddSatisfy("A::h", "A::r")
            .AddSatisfy("A::h", "A::r", s => s with { SatisfyingFeature = "x", IsAsserted = true })
            .AddRequirementConstraint("A::r", "assume", "x > 0")
            .AddRequirementConstraint("A::r", "require", "x < 9", "upper")
            .AddTransition("A::S", "idle", "busy")
            .AddTransition("A::S", "busy", "idle", t => t with { Trigger = "Stop" })
            .AddEntryTransition("A::S", "idle")
            .AddImport("A", "L::*")
            .AddImport("A", "L::*", i => i with { IsRecursive = true })
            .AddDocumentation("A::B", "B.")
            .AddDocumentation("A::B", "B!", d => d with { Replace = true })
            .AddVerify("A::t", "A::r")
            .AddMetadata("A", "Safety")
            .AddMetadata("A", "Safety", m => m with { Shorthand = true })
            .AddMetadataPrefix("A::B", "Safety")
            .AddComment("A", "c")
            .AddComment("A", "c", c => c with { About = ["A::B"] })
            .AddNote("A::B", "n")
            .AddFirst("A::act", "start")
            .AddThen("A::act", "done")
            .AddThenMember("A::act", "action", "b")
            .AddAccept("A::act", "sig")
            .AddSend("A::act", "ping")
            .AddSend("A::act", "ping", "peer")
            .AddAssign("A::act", "x", "1")
            .AddIf("A::act", "x == 0", then)
            .AddIf("A::act", "x == 0", then, otherwise)
            .AddWhile("A::act", "x < 3", then)
            .AddWhile("A::act", "x < 3", then, "x > 9")
            .AddLoop("A::act", then)
            .AddLoop("A::act", then, "x > 9")
            .AddFor("A::act", "i", "items", then)
            .AddTerminate("A::act")
            .AddTerminate("A::act", "self")
            .AddGuardedThen("A::act", "x > 0", "done")
            .AddElse("A::act", "start");
        IReadOnlyList<Edit.AddSequence> body = [new Edit.AddSequence(string.Empty, string.Empty) { MemberKind = "assign", Target = "x", Value = "1" }, Edit.AddSequence.Then(string.Empty, "done")];

        Assert.Equal(
            [
                new Edit.SetValue("A::x", "1"),
                new Edit.Rename("A::B", "C"),
                new Edit.Delete("A::Old"),
                new Edit.Delete("A::Older") { Cascade = true },
                new Edit.Move("A::W", "A::P"),
                new Edit.AddMember("A", "part", "engine"),
                new Edit.AddMember("A", "part", "spare") { Type = "Engine", Multiplicity = "[0..1]" },
                new Edit.AddMember("A::act", string.Empty, "speed") { Direction = "in", Type = "Real" },
                new Edit.AddMember("A::act", string.Empty, "done") { Direction = "out" },
                new Edit.AddConnection("A", "allocation", "a", "b"),
                new Edit.AddConnection("A", "flow", "a", "b") { Name = "f" },
                new Edit.AddSatisfy("A::h", "A::r"),
                new Edit.AddSatisfy("A::h", "A::r") { SatisfyingFeature = "x", IsAsserted = true },
                new Edit.AddRequirementConstraint("A::r", "assume", "x > 0"),
                new Edit.AddRequirementConstraint("A::r", "require", "x < 9") { Name = "upper" },
                new Edit.AddTransition("A::S", "idle", "busy"),
                new Edit.AddTransition("A::S", "busy", "idle") { Trigger = "Stop" },
                new Edit.AddTransition("A::S", string.Empty, "idle") { Initial = true },
                new Edit.AddImport("A", "L::*"),
                new Edit.AddImport("A", "L::*") { IsRecursive = true },
                new Edit.AddDocumentation("A::B", "B."),
                new Edit.AddDocumentation("A::B", "B!") { Replace = true },
                new Edit.AddVerify("A::t", "A::r"),
                new Edit.AddMetadata("A", "Safety"),
                new Edit.AddMetadata("A", "Safety") { Shorthand = true },
                new Edit.AddMetadataPrefix("A::B", "Safety"),
                new Edit.AddComment("A", "c"),
                new Edit.AddComment("A", "c") { About = ["A::B"] },
                new Edit.AddNote("A::B", "n"),
                new Edit.AddSequence("A::act", "first") { Ref = "start" },
                new Edit.AddSequence("A::act", "then") { Ref = "done" },
                new Edit.AddSequence("A::act", "then") { MemberKind = "action", MemberName = "b" },
                new Edit.AddSequence("A::act", "then") { MemberKind = "accept", Parameter = "sig" },
                new Edit.AddSequence("A::act", "then") { MemberKind = "send", Value = "ping" },
                new Edit.AddSequence("A::act", "then") { MemberKind = "send", Value = "ping", Target = "peer" },
                new Edit.AddSequence("A::act", "then") { MemberKind = "assign", Target = "x", Value = "1" },
                new Edit.AddSequence("A::act", "then") { MemberKind = "if", Condition = "x == 0", Body = body },
                new Edit.AddSequence("A::act", "then")
                {
                    MemberKind = "if",
                    Condition = "x == 0",
                    Body = body,
                    ElseBody = [new Edit.AddSequence(string.Empty, string.Empty) { MemberKind = "send", Value = "ping", Target = "peer" }],
                },
                new Edit.AddSequence("A::act", "then") { MemberKind = "while", Condition = "x < 3", Body = body },
                new Edit.AddSequence("A::act", "then") { MemberKind = "while", Condition = "x < 3", Body = body, Until = "x > 9" },
                new Edit.AddSequence("A::act", "then") { MemberKind = "loop", Body = body },
                new Edit.AddSequence("A::act", "then") { MemberKind = "loop", Body = body, Until = "x > 9" },
                new Edit.AddSequence("A::act", "then") { MemberKind = "for", Parameter = "i", Value = "items", Body = body },
                new Edit.AddSequence("A::act", "then") { MemberKind = "terminate" },
                new Edit.AddSequence("A::act", "then") { MemberKind = "terminate", Value = "self" },
                new Edit.AddSequence("A::act", "if") { Condition = "x > 0", Ref = "done" },
                new Edit.AddSequence("A::act", "else") { Ref = "start" },
            ],
            editor.Edits);
        Assert.Same(model, editor.Model);
        Assert.False(editor.IsApplied);
    }

    [Fact]
    public void ABodyWritesItsFirstStatementWithoutThenAndIsReadWhenItIsPassed()
    {
        var inner = new Editor.Body().AddTerminate();
        var body = new Editor.Body()
            .AddFirst("start")
            .AddAccept("sig")
            .AddAssign("x", "1")
            .AddThenMember("action", "b")
            .AddAction("c")
            .AddIf("x > 0", inner, new Editor.Body().AddLoop(inner, "x > 9"))
            .AddWhile("x < 3", inner)
            .AddFor("i", "items", inner)
            .AddLoop(inner)
            .AddSend("ping")
            .AddGuardedThen("x > 0", "done")
            .AddElse("start")
            .AddThen("done");
        var statement = Edit.AddSequence.If("A", "x == 0", body.Items);
        body.AddThen("later");
        inner.AddAssign("y", "2");

        Assert.Equal(
            ["first", "then", "then", "then", string.Empty, "then", "then", "then", "then", "then", "if", "else", "then"],
            statement.Body.Select(item => item.Keyword));
        Assert.Equal([string.Empty], statement.Body[5].Body.Select(item => item.Keyword));
        Assert.Equal(("loop", "x > 9", 1), (statement.Body[5].ElseBody[0].MemberKind, statement.Body[5].ElseBody[0].Until, statement.Body[5].ElseBody[0].Body.Count));
        Assert.Equal(13, statement.Body.Count);
        Assert.Equal(14, body.Items.Count);
        Assert.Equal([string.Empty], new Editor.Body().AddSend("ping").Items.Select(item => item.Keyword));
        Assert.Equal([Edit.AddSequence.Then("ignored", "x")], new Editor.Body().Add(Edit.AddSequence.Then("ignored", "x")).Items);
        Assert.Throws<InvalidOperationException>(() => Edit.AddSequence.First("A", "x").WithoutThen());
        Assert.Throws<InvalidOperationException>(() => (Edit.AddSequence.Then("A", "x") with { Multiplicity = "[1]" }).WithoutThen());
        Assert.Throws<ArgumentNullException>(() => new Editor.Body().Add(null!));
        Assert.Throws<ArgumentNullException>(() => new Editor.Body().AddIf("x", null!));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnEditorAppliesWhatItHeldWhenAskedOnceAndRefusesEditsWhileAndAfter(ClientWire wire)
    {
        var release = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var arrived = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var answers = new Queue<Wire.ApplyEditsResponse>([CompleteEditingWire.Refusal, CompleteEditingWire.Single]);
        using var handler = Serving(wire);
        handler.Respond = async (sent, token) =>
        {
            if (sent.Method != "ApplyEdits")
            {
                return handler.Refuse(StatusCode.Unimplemented, "not faked");
            }

            if (answers.Count == 1)
            {
                arrived.TrySetResult();
                await release.Task.WaitAsync(token);
            }

            return handler.Answer(answers.Dequeue());
        };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var editor = connection.ModelByHash(Hash).Edit().SetValue("A::x", "1");

        await Assert.ThrowsAsync<EditException>(() => editor.ApplyAsync(cancellationToken: Token));
        editor.Rename("A::B", "C");
        var applying = editor.ApplyAsync(cancellationToken: Token);
        await arrived.Task.WaitAsync(Token);
        var during = Assert.Throws<InvalidOperationException>(() => editor.Delete("A::D"));
        var twice = await Assert.ThrowsAsync<InvalidOperationException>(() => editor.ApplyAsync(cancellationToken: Token));
        release.SetResult();
        var result = await applying;
        var after = Assert.Throws<InvalidOperationException>(() => editor.AddNote("A::B", "n"));
        var again = await Assert.ThrowsAsync<InvalidOperationException>(() => editor.ApplyAsync(cancellationToken: Token));

        Assert.Equal(CompleteEditingWire.Edited, result.Content);
        Assert.True(editor.IsApplied);
        Assert.Contains("being applied", during.Message, StringComparison.Ordinal);
        Assert.Contains("being applied", twice.Message, StringComparison.Ordinal);
        Assert.Contains("was applied", after.Message, StringComparison.Ordinal);
        Assert.Contains("was applied", again.Message, StringComparison.Ordinal);
        Assert.Equal([new Edit.SetValue("A::x", "1"), new Edit.Rename("A::B", "C")], editor.Edits);
        var sent = handler.RequestsFor("ApplyEdits").Select(request => request.Message(Wire.ApplyEditsRequest.Parser).Operations.Count);
        Assert.Equal([1, 2], sent);
    }

    public static TheoryData<ClientWire, string> GatedEdits()
    {
        var data = new TheoryData<ClientWire, string>();
        foreach (var wire in new[] { ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson })
        {
            foreach (var name in Gated.Keys)
            {
                data.Add(wire, name);
            }
        }

        return data;
    }

    [Theory]
    [MemberData(nameof(GatedEdits))]
    public async Task AnEditNeedingACapabilityTheServiceLacksIsRefusedBeforeAnythingIsSent(ClientWire wire, string name)
    {
        var (edit, capability) = Gated[name];

        Assert.Equal(capability, await RefusedAsync(wire, [capability], model => model.ApplyEditsAsync([edit], cancellationToken: Token)));
        Assert.Equal(Capabilities.ApplyEdits, await RefusedAsync(wire, [Capabilities.ApplyEdits, capability], model => model.ApplyEditsAsync([edit], cancellationToken: Token)));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnEditNeedingNoMoreIsSentWithEveryOtherCapabilityWithheld(ClientWire wire)
    {
        Edit[] ungated = [new Edit.SetValue("A::x", "1"), new Edit.Rename("A::B", "C")];
        Edit[] plain =
        [
            Edit.AddSequence.Then("A", "done"),
            Edit.AddSequence.First("A", "start"),
            Edit.AddSequence.ThenMember("A", "action", "b"),
            new Edit.AddMember("A", "objective", "named"),
            new Edit.AddMember("A", "part", "p") { Type = "P", Multiplicity = "[1]", Value = "new()", Specializes = ["Q"] },
        ];
        using var handler = new FakeServiceHandler(wire) { Capabilities = [Capabilities.ApplyEdits] }.On("ApplyEdits", Wire.ApplyEditsRequest.Parser, request => request.Operations.Count == 0
            ? new Wire.ApplyEditsResponse { Error = "no edit operations", Failure = Wire.EditFailure.NoOperations }
            : CompleteEditingWire.Bare);
        using var authoring = new FakeServiceHandler(wire) { Capabilities = [Capabilities.ApplyEdits, Capabilities.Authoring, Capabilities.SequenceAuthoring] }
            .On("ApplyEdits", Wire.ApplyEditsRequest.Parser, _ => CompleteEditingWire.Bare);
        await using var connection = await FakeConnection.OpenAsync(handler);
        await using var authored = await FakeConnection.OpenAsync(authoring);

        await connection.ModelByHash(Hash).ApplyEditsAsync(ungated, cancellationToken: Token);
        await Assert.ThrowsAsync<EditException>(() => connection.ModelByHash(Hash).ApplyEditsAsync([], cancellationToken: Token));
        foreach (var edit in plain)
        {
            await authored.ModelByHash(Hash).ApplyEditsAsync([edit], cancellationToken: Token);
        }

        Assert.Equal(2, handler.RequestsFor("ApplyEdits").Count);
        Assert.Empty(handler.RequestsFor("ApplyEdits")[1].Message(Wire.ApplyEditsRequest.Parser).Operations);
        Assert.Equal(plain.Length, authoring.RequestsFor("ApplyEdits").Count);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheBatchIsJudgedWholeInTheServicesOrderAndANamedDocumentLast(ClientWire wire)
    {
        Edit[] batch = [new Edit.AddNote("A::B", "n"), new Edit.AddConnection("A", "flow", "a", "b"), new Edit.AddMember("A", "part", "p") { IsAbstract = true }, new Edit.Delete("A::B")];
        var named = new EditOptions { InDocument = "b.sysml" };

        Assert.Equal(Capabilities.Authoring, await RefusedAsync(wire, [.. Internal.EditWire.GateOrder], model => model.ApplyEditsAsync(batch, cancellationToken: Token)));
        Assert.Equal(
            Capabilities.ConnectionAuthoring,
            await RefusedAsync(wire, [Capabilities.CommentAuthoring, Capabilities.MemberModifiers, Capabilities.ConnectionAuthoring], model => model.ApplyEditsAsync(batch, cancellationToken: Token)));
        Assert.Equal(
            Capabilities.MemberModifiers,
            await RefusedAsync(wire, [Capabilities.CommentAuthoring, Capabilities.MemberModifiers], model => model.ApplyEditsAsync(batch, cancellationToken: Token)));
        Assert.Equal(
            Capabilities.CommentAuthoring,
            await RefusedAsync(wire, [Capabilities.CommentAuthoring, Capabilities.EditDocuments], model => model.ApplyEditsAsync(batch, named, Token)));
        Assert.Equal(Capabilities.EditDocuments, await RefusedAsync(wire, [Capabilities.EditDocuments], model => model.ApplyEditsAsync(batch, named, Token)));
        Assert.Equal(Capabilities.ApplyEdits, await RefusedAsync(wire, [Capabilities.ApplyEdits, Capabilities.EditDocuments], model => model.ApplyEditsAsync([], named, Token)));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AStatementNestedAtAnyDepthIsJudgedAndSentAsDeepAsTheServiceReads(ClientWire wire)
    {
        var deepest = Edit.AddSequence.Then(string.Empty, "x");
        for (var level = 0; level < Edit.AddSequence.MaxDepth; level++)
        {
            deepest = Edit.AddSequence.ThenMember(string.Empty, "action", $"a{level}") with { Body = [deepest] };
        }

        var top = deepest with { Owner = "A" };
        using var handler = Serving(wire);
        handler.Respond = (sent, _) => Task.FromResult(handler.Answer(CompleteEditingWire.Bare));
        await using var connection = await FakeConnection.OpenAsync(handler);

        await connection.ModelByHash(Hash).ApplyEditsAsync([top], cancellationToken: Token);
        var refused = await RefusedAsync(wire, [Capabilities.ActionBodyStatementAuthoring], model => model.ApplyEditsAsync([top], cancellationToken: Token));
        var bare = await RefusedAsync(
            wire,
            [Capabilities.ActionBodyStatementAuthoring],
            model => model.ApplyEditsAsync([Edit.AddSequence.ThenMember("A", "action", "b") with { Body = [Edit.AddSequence.Then(string.Empty, "x")] }], cancellationToken: Token));

        Assert.Equal((Capabilities.ActionBodyStatementAuthoring, Capabilities.ActionBodyStatementAuthoring), (refused, bare));
        var sent = Deep(Assert.Single(handler.RequestsFor("ApplyEdits"))).Operations[0].AddSequence;
        var depth = 0;
        for (; sent.Body.Count > 0; depth++)
        {
            sent = sent.Body[0];
        }

        Assert.Equal((Edit.AddSequence.MaxDepth, "x"), (depth, sent.Ref));
        Assert.Equal(top, top with { });
        Assert.Equal(top.GetHashCode(), (deepest with { Owner = "A" }).GetHashCode());
        Assert.StartsWith("AddSequence {", top.ToString(), StringComparison.Ordinal);
    }

    [Fact]
    public void AnItemNestedDeeperThanTheServiceReadsCannotBeBuilt()
    {
        var item = Edit.AddSequence.Then(string.Empty, "x");
        for (var level = 0; level < Edit.AddSequence.MaxDepth; level++)
        {
            item = Edit.AddSequence.Loop(string.Empty, [item]);
        }

        var deep = Assert.Throws<ArgumentException>(() => Edit.AddSequence.Loop(string.Empty, [item]));
        Assert.Throws<ArgumentException>(() => Edit.AddSequence.If(string.Empty, "c", [], [item]));
        Assert.Throws<ArgumentException>(() => new Edit.AddSequence("A", "then") { ElseBody = [item] });
        Assert.Throws<ArgumentException>(() => new Editor.Body().AddWhile("c", new Editor.Body().Add(item)));

        Assert.Contains($"the {Edit.AddSequence.MaxDepth} levels the service reads", deep.Message, StringComparison.Ordinal);
        Assert.Equal(Edit.AddSequence.MaxDepth + 1, item.Height);
        Assert.Equal(Edit.AddSequence.MaxDepth, item.Body[0].Height);
        Assert.Equal(1, (item with { Body = [], ElseBody = [] }).Height);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnEmptyOptionalPartOrDocumentIsRefusedBeforeAnyCapabilityIsJudged(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = [] };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var nested = Edit.AddSequence.If("A", "c", [Edit.AddSequence.Then(string.Empty, "x")], [Edit.AddSequence.Assign(string.Empty, "x", "1") with { Via = string.Empty }]);

        var member = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ApplyEditsAsync([new Edit.SetValue("A", "1"), new Edit.AddMember("A", "part", "p") { Type = string.Empty }], cancellationToken: Token));
        var deep = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ApplyEditsAsync([nested], cancellationToken: Token));
        var document = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ApplyEditsAsync([], new EditOptions { InDocument = string.Empty }, Token));
        Edit[] empties =
        [
            new Edit.AddMember("A", "part", "p") { Multiplicity = string.Empty },
            new Edit.AddMember("A", "part", "p") { Value = string.Empty },
            new Edit.AddMember("A", "part", "p") { Direction = string.Empty },
            new Edit.AddMember("A", "part", "p") { Doc = string.Empty },
            new Edit.AddMember("A", "part", "p") { BodyExpression = string.Empty },
            new Edit.AddConnection("A", "flow", "a", "b") { Name = string.Empty },
            new Edit.AddConnection("A", "flow", "a", "b") { Type = string.Empty },
            new Edit.AddSatisfy("A", "R") { SatisfyingFeature = string.Empty },
            new Edit.AddRequirementConstraint("A", "require", "x") { Name = string.Empty },
            new Edit.AddTransition("A", "s", "t") { Name = string.Empty },
            new Edit.AddTransition("A", "s", "t") { Trigger = string.Empty },
            new Edit.AddTransition("A", "s", "t") { Guard = string.Empty },
            new Edit.AddTransition("A", "s", "t") { Effect = string.Empty },
            new Edit.AddImport("A", "L::*") { Visibility = string.Empty },
            new Edit.AddDocumentation("A", "d") { Name = string.Empty },
            new Edit.AddDocumentation("A", "d") { Locale = string.Empty },
            new Edit.AddMetadata("A", "M") { Name = string.Empty },
            new Edit.AddComment("A", "c") { Name = string.Empty },
            new Edit.AddComment("A", "c") { Locale = string.Empty },
            .. SequenceParts.Select(property => (Edit)Sequence(property)),
        ];
        foreach (var empty in empties)
        {
            await Assert.ThrowsAsync<InvalidRequestException>(() => model.ApplyEditsAsync([empty], cancellationToken: Token));
        }

        await Assert.ThrowsAsync<ArgumentNullException>(() => model.ApplyEditsAsync(null!, cancellationToken: Token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.ApplyEditsAsync([null!], cancellationToken: Token));
        Assert.Contains("AddMember at edits[1] sets Type to empty text", member.Message, StringComparison.Ordinal);
        Assert.Contains("edits[0].ElseBody[0] sets Via", deep.Message, StringComparison.Ordinal);
        Assert.Contains("EditOptions.InDocument is empty", document.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASingleDocumentModelAnswersItsNotationInContentAndInItsOneDocument(ClientWire wire)
    {
        using var handler = Serving(wire)
            .Parsing(Hash, FakeModels.Symbol("Demo", "Demo", "package"))
            .On("ApplyEdits", Wire.ApplyEditsRequest.Parser, request => request.Operations[0].SetValue.Target switch
            {
                "bare" => CompleteEditingWire.Bare,
                "emptied" => new Wire.ApplyEditsResponse { Documents = { new Wire.EditedDocument { Name = "a.sysml" } } },
                _ => CompleteEditingWire.Single,
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var result = await model.Edit().SetValue("full", "1").SetValue("x", "2").ApplyAsync(cancellationToken: Token);
        var bare = await model.ApplyEditsAsync([new Edit.SetValue("bare", "1")], cancellationToken: Token);
        var emptied = await model.ApplyEditsAsync([new Edit.SetValue("emptied", "1")], cancellationToken: Token);

        Assert.Equal((CompleteEditingWire.Edited, false), (result.Content, result.HasSeveralDocuments));
        Assert.Equal([new EditedDocument("a.sysml", CompleteEditingWire.Edited)], result.Documents);
        Assert.Equal(
            [
                new AppliedEdit(0, "Demo::unitMass", 41, 14, "1000.0[SI::kg]", "1050.0[SI::kg]", "a.sysml"),
                new AppliedEdit(1, "Demo", 60, 0, string.Empty, "// note\n", "a.sysml"),
            ],
            result.Applied);
        Assert.Equal(ModelWireDiagnostic(), Assert.Single(result.Diagnostics));
        Assert.Equal((CompleteEditingWire.Edited, 0), (bare.Content, bare.Documents.Count));
        Assert.Null(Assert.Single(bare.Applied).Document);
        Assert.Equal((string.Empty, false), (emptied.Content, emptied.HasSeveralDocuments));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AModelOfSeveralDocumentsAnswersItsDocumentsAloneAndAHashedModelIsReadFromTheAnswer(ClientWire wire)
    {
        using var handler = Serving(wire)
            .On("ParseSources", Wire.ParseSourcesRequest.Parser, _ => new Wire.ParseSourcesResponse { ModelHash = Hash })
            .On("ApplyEdits", Wire.ApplyEditsRequest.Parser, request => request.Operations[0].SetValue.Target switch
            {
                "single" => CompleteEditingWire.Single,
                "emptied" => new Wire.ApplyEditsResponse { Documents = { new Wire.EditedDocument { Name = "engine_user.sysml" } } },
                "nothing" => new Wire.ApplyEditsResponse(),
                _ => CompleteEditingWire.Several,
            });
        using var predating = new FakeServiceHandler(wire) { Capabilities = [Capabilities.ApplyEdits] }.On("ApplyEdits", Wire.ApplyEditsRequest.Parser, _ => new Wire.ApplyEditsResponse());
        await using var connection = await FakeConnection.OpenAsync(handler);
        await using var old = await FakeConnection.OpenAsync(predating);
        var several = await connection.ParseSourcesAsync([SourceDocument.Inline("package A;", "a.sysml"), SourceDocument.Inline("package B;", "b.sysml")], cancellationToken: Token);
        var hashed = connection.ModelByHash(Hash);

        var result = await several.ApplyEditsAsync([new Edit.SetValue("several", "1")], new EditOptions { InDocument = "b.sysml" }, Token);
        var emptied = await several.ApplyEditsAsync([new Edit.SetValue("emptied", "1")], cancellationToken: Token);
        var inferred = await hashed.ApplyEditsAsync([new Edit.SetValue("several", "1")], cancellationToken: Token);
        var single = await hashed.ApplyEditsAsync([new Edit.SetValue("single", "1"), new Edit.SetValue("x", "2")], cancellationToken: Token);
        var nothing = await hashed.ApplyEditsAsync([new Edit.SetValue("nothing", "1")], cancellationToken: Token);
        var unnamed = await hashed.ApplyEditsAsync([new Edit.SetValue("emptied", "1")], cancellationToken: Token);
        var older = await old.ModelByHash(Hash).ApplyEditsAsync([new Edit.SetValue("x", "1")], cancellationToken: Token);

        Assert.Equal((null, true), (result.Content, result.HasSeveralDocuments));
        Assert.Equal(["engine_library.sysml", "engine_user.sysml"], result.Documents.Select(document => document.Name));
        Assert.Equal(["engine_library.sysml", "engine_user.sysml"], result.Applied.Select(applied => applied.Document));
        Assert.Equal(result, inferred);
        Assert.Equal((null, "engine_user.sysml", string.Empty), (emptied.Content, Assert.Single(emptied.Documents).Name, emptied.Documents[0].Content));
        Assert.Equal(CompleteEditingWire.Edited, single.Content);
        Assert.Equal((null, 0), (nothing.Content, nothing.Documents.Count));
        Assert.True(unnamed.HasSeveralDocuments);
        Assert.Equal((string.Empty, false), (older.Content, older.HasSeveralDocuments));
        Assert.Equal(new Wire.ApplyEditsRequest { ModelHash = Hash, Document = "b.sysml", AcceptDocuments = true, Operations = { new Wire.EditOperation { SetValue = new Wire.SetValueEdit { Target = "several", Value = "1" } } } }, handler.RequestsFor("ApplyEdits")[0].Message(Wire.ApplyEditsRequest.Parser));
        var refused = await Assert.ThrowsAsync<InvalidOperationException>(() => result.SaveAsync(Path.Combine(directory, "model.sysml"), cancellationToken: Token));
        Assert.Contains("EditedDocument.SaveAsync", refused.Message, StringComparison.Ordinal);
        Assert.Empty(Directory.GetFileSystemEntries(directory));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARefusalIsAnEditExceptionKeepingItsKindDiagnosticsAndReferrers(ClientWire wire)
    {
        using var handler = Serving(wire).On("ApplyEdits", Wire.ApplyEditsRequest.Parser, request => request.Operations.Count == 0
            ? new Wire.ApplyEditsResponse { Error = "no edit operations", Failure = Wire.EditFailure.NoOperations }
            : request.Operations[0].Delete is null
                ? new Wire.ApplyEditsResponse { Error = "a refusal of a newer kind", Failure = (Wire.EditFailure)99 }
                : CompleteEditingWire.Refusal);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);

        var refused = await Assert.ThrowsAsync<EditException>(() => model.Edit().Delete("EngineLibrary::Engine").ApplyAsync(cancellationToken: Token));
        var empty = await Assert.ThrowsAsync<EditException>(() => model.Edit().ApplyAsync(cancellationToken: Token));
        var newer = await Assert.ThrowsAsync<EditException>(() => model.ApplyEditsAsync([new Edit.Rename("A", "B")], cancellationToken: Token));

        Assert.Equal(("EngineLibrary::Engine is referred to by EngineUser::Car::motor", EditFailure.DeleteReferenced, Model.ApplyEditsMethod), (refused.Message, refused.Failure, refused.Method));
        Assert.Equal(ModelWireDiagnostic(), Assert.Single(refused.Diagnostics));
        Assert.Equal(["EngineUser::Car::motor (engine_user.sysml)", "EngineLibrary::spare"], refused.ReferringElements);
        Assert.Equal([new Referrer("EngineLibrary::spare", "engine_library.sysml"), new Referrer("EngineUser::Car::motor", "engine_user.sysml")], refused.Referrers);
        Assert.IsAssignableFrom<ModelException>(refused);
        Assert.Equal((EditFailure.NoOperations, 0, 0), (empty.Failure, empty.ReferringElements.Count, empty.Referrers.Count));
        Assert.Equal((EditFailure)99, newer.Failure);
        Assert.Empty(handler.RequestsFor("ApplyEdits")[1].Message(Wire.ApplyEditsRequest.Parser).Operations);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AMalformedAnswerIsATransportFailure(ClientWire wire)
    {
        var malformed = new Dictionary<string, (Wire.ApplyEditsResponse Answer, string Says)>(StringComparer.Ordinal)
        {
            ["refusal and content"] = (new() { Error = "no", Content = "x" }, "a refusal beside edited notation"),
            ["refusal and documents"] = (new() { Error = "no", Documents = { new Wire.EditedDocument { Name = "a" } } }, "a refusal beside edited notation"),
            ["refusal and applied"] = (new() { Error = "no", Applied = { new Wire.AppliedEdit() } }, "a refusal beside edited notation"),
            ["kind alone"] = (new() { Content = "x", Failure = Wire.EditFailure.UnknownTarget }, "the refusal kind UnknownTarget without the refusal"),
            ["referring alone"] = (new() { Content = "x", ReferringElements = { "A" } }, "though it refused nothing"),
            ["referrers alone"] = (new() { Content = "x", Referrers = { new Wire.Referrer { Name = "A", Document = "a" } } }, "though it refused nothing"),
            ["unnamed document"] = (new() { Documents = { new Wire.EditedDocument { Content = "x" } } }, "documents[0] without its name"),
            ["repeated document"] = (new() { Documents = { new Wire.EditedDocument { Name = "a" }, new Wire.EditedDocument { Name = "a" } } }, "documents[1] repeating the document \"a\""),
            ["index beyond"] = (new() { Content = "x", Applied = { new Wire.AppliedEdit { OperationIndex = 1 } } }, "applied[0] for edit 1, which a batch of 1 has none of"),
            ["index negative"] = (new() { Content = "x", Applied = { new Wire.AppliedEdit { OperationIndex = -1 } } }, "applied[0] for edit -1"),
            ["offset negative"] = (new() { Content = "x", Applied = { new Wire.AppliedEdit { Offset = -1 } } }, "applied[0] at offset -1"),
            ["length negative"] = (new() { Content = "x", Applied = { new Wire.AppliedEdit { Length = -2 } } }, "for -2 bytes"),
            ["unlisted document"] = (
                new() { Content = "x", Documents = { new Wire.EditedDocument { Name = "a", Content = "x" } }, Applied = { new Wire.AppliedEdit { Document = "b" } } },
                "applied[0] in the document \"b\", which documents does not list"),
            ["two documents"] = (
                new() { Content = "x", Documents = { new Wire.EditedDocument { Name = "a", Content = "x" }, new Wire.EditedDocument { Name = "b", Content = "x" } } },
                "documents other than the one document"),
            ["other notation"] = (new() { Content = "x", Documents = { new Wire.EditedDocument { Name = "a", Content = "y" } } }, "documents other than the one document"),
        };
        using var handler = Serving(wire)
            .On("ParseSources", Wire.ParseSourcesRequest.Parser, _ => new Wire.ParseSourcesResponse { ModelHash = Hash })
            .On("ApplyEdits", Wire.ApplyEditsRequest.Parser, request => request.Operations[0].SetValue.Target == "several"
                ? new Wire.ApplyEditsResponse { Content = "x", Documents = { new Wire.EditedDocument { Name = "a" } } }
                : malformed[request.Operations[0].SetValue.Target].Answer);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var several = await connection.ParseSourcesAsync([SourceDocument.Inline("package A;", "a"), SourceDocument.Inline("package B;", "b")], cancellationToken: Token);

        foreach (var (name, (_, says)) in malformed)
        {
            var error = await Assert.ThrowsAsync<TransportException>(() => model.ApplyEditsAsync([new Edit.SetValue(name, "1")], cancellationToken: Token));
            Assert.Equal(StatusCode.Internal, error.StatusCode);
            Assert.Contains(says, error.Message, StringComparison.Ordinal);
        }

        var content = await Assert.ThrowsAsync<TransportException>(() => several.ApplyEditsAsync([new Edit.SetValue("several", "1")], cancellationToken: Token));
        Assert.Contains("notation in content for a model of several documents", content.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AMissingModelIsModelNotFoundAndTheServicesOtherRefusalsKeepTheirStatus(ClientWire wire)
    {
        using var missing = Serving(wire).Refusing("ApplyEdits", StatusCode.NotFound, "model h is no longer cached: parse it again before editing it");
        using var unknown = Serving(wire).Refusing("ApplyEdits", StatusCode.InvalidArgument, "document \"c.sysml\" is not one of the model's: it has a.sysml, b.sysml");
        using var several = Serving(wire).Refusing("ApplyEdits", StatusCode.FailedPrecondition, "the model has 2 documents, and the request reads only content");
        await using var gone = await FakeConnection.OpenAsync(missing);
        await using var named = await FakeConnection.OpenAsync(unknown);
        await using var refused = await FakeConnection.OpenAsync(several);
        Edit[] batch = [new Edit.Rename("A", "B")];

        var notFound = await Assert.ThrowsAsync<ModelNotFoundException>(() => gone.ModelByHash(Hash).ApplyEditsAsync(batch, cancellationToken: Token));
        var invalid = await Assert.ThrowsAsync<ServiceException>(() => named.ModelByHash(Hash).ApplyEditsAsync(batch, new EditOptions { InDocument = "c.sysml" }, Token));
        var precondition = await Assert.ThrowsAsync<ServiceException>(() => refused.ModelByHash(Hash).ApplyEditsAsync(batch, new EditOptions { AcceptDocuments = false }, Token));

        Assert.Equal((StatusCode.NotFound, Model.ApplyEditsMethod), (notFound.StatusCode, notFound.Method));
        Assert.Equal((StatusCode.InvalidArgument, typeof(ServiceException)), (invalid.StatusCode, invalid.GetType()));
        Assert.Equal((StatusCode.FailedPrecondition, typeof(ServiceException)), (precondition.StatusCode, precondition.GetType()));
        Assert.False(Assert.Single(several.RequestsFor("ApplyEdits")).Message(Wire.ApplyEditsRequest.Parser).AcceptDocuments);
    }

    [Fact]
    public async Task TheEditedNotationIsSavedWholeAndNeverOverAnExistingFileUnlessAsked()
    {
        var path = Path.Combine(directory, "model.sysml");
        var result = new EditResult("package P {\r\n    // \u00e9\r\n}\n", [], [new EditedDocument("<content>", "package P;")], []);
        var document = new EditedDocument("b.sysml", "package B;\n");

        var written = await result.SaveAsync(path, cancellationToken: Token);
        var refused = await Assert.ThrowsAsync<UnwritableException>(() => document.SaveAsync(path, cancellationToken: Token));
        var kept = await File.ReadAllBytesAsync(path, Token);
        var other = await document.SaveAsync(Path.Combine(directory, "b.sysml"), cancellationToken: Token);
        await document.SaveAsync(path, new SaveOptions { Overwrite = true }, Token);

        Assert.Equal(Path.GetFullPath(path), written);
        Assert.Equal("package P {\r\n    // \u00e9\r\n}\n"u8.ToArray(), kept);
        Assert.Equal(path, refused.Path);
        Assert.Equal(Path.Combine(Path.GetFullPath(directory), "b.sysml"), other);
        Assert.Equal("package B;\n", await File.ReadAllTextAsync(path, Token));
        Assert.Equal(["b.sysml", "model.sysml"], Directory.GetFileSystemEntries(directory).Select(Path.GetFileName).Order(StringComparer.Ordinal));
        await Assert.ThrowsAsync<ArgumentException>(() => result.SaveAsync(string.Empty, cancellationToken: Token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => document.SaveAsync(null!, cancellationToken: Token));
    }

    [Fact]
    public async Task AFailedOrCancelledSaveLeavesNothingAndAnExistingFileAsItWas()
    {
        var path = Path.Combine(directory, "model.sysml");
        var fresh = Path.Combine(directory, "fresh.sysml");
        await File.WriteAllTextAsync(path, "old\n", Token);
        var result = new EditResult("new\n", [], [], []);
        var document = new EditedDocument("a.sysml", "new\n");
        using var cancelled = new CancellationTokenSource();
        await cancelled.CancelAsync();

        await Assert.ThrowsAsync<IOException>(() => result.PublishAsync(path, new SaveOptions { Overwrite = true }, _ => throw new IOException("disk full"), Token));
        await Assert.ThrowsAsync<IOException>(() => document.PublishAsync(fresh, null, _ => throw new IOException("disk full"), Token));
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => result.SaveAsync(fresh, null, cancelled.Token));
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => document.SaveAsync(path, new SaveOptions { Overwrite = true }, cancelled.Token));

        Assert.Equal("old\n", await File.ReadAllTextAsync(path, Token));
        Assert.Equal(["model.sysml"], Directory.GetFileSystemEntries(directory).Select(Path.GetFileName));
    }

    [Fact]
    public void EditsAreImmutableValuesComparedByContentThatRefuseNull()
    {
        var specializes = new List<string> { "Base" };
        var member = new Edit.AddMember("A", "part def", "B") { Specializes = specializes };
        specializes.Add("Other");
        var values = new List<MetadataFeatureValue> { new("f", "1") };
        var metadata = new Edit.AddMetadata("A", "M") { Values = values };
        values.Clear();

        Assert.Equal(["Base"], member.Specializes);
        Assert.Single(metadata.Values);
        Assert.Equal(member, new Edit.AddMember("A", "part def", "B") { Specializes = ["Base"] });
        Assert.Equal(member.GetHashCode(), new Edit.AddMember("A", "part def", "B") { Specializes = ["Base"] }.GetHashCode());
        Assert.NotEqual(member, member with { Specializes = ["Base", "Other"] });
        Assert.NotEqual<Edit>(new Edit.SetValue("A", "1"), new Edit.Rename("A", "1"));
        Assert.Equal(new Edit.Delete("A"), new Edit.Delete("A", cascade: false));
        Assert.Equal(new MetadataFeatureValue("f", "1"), new MetadataFeatureValue("f", "1"));
        Assert.Contains("Kind = part def", member.ToString(), StringComparison.Ordinal);
        Assert.Throws<ArgumentNullException>(() => new Edit.SetValue(null!, "1"));
        Assert.Throws<ArgumentNullException>(() => new Edit.SetValue("A", "1") with { Value = null! });
        Assert.Throws<ArgumentNullException>(() => new Edit.AddMember("A", "part", "p") { Specializes = null! });
        Assert.Throws<ArgumentException>(() => new Edit.AddComment("A", "c") { About = [null!] });
        Assert.Throws<ArgumentException>(() => new Edit.AddMetadata("A", "M") { Values = [null!] });
        Assert.Throws<ArgumentNullException>(() => new MetadataFeatureValue("f", null!));
        Assert.Throws<ArgumentNullException>(() => Edit.AddSequence.First("A", null!));
        Assert.Throws<ArgumentNullException>(() => Edit.AddSequence.Loop("A", null!));
        Assert.Throws<ArgumentException>(() => new AppliedEdit(0, "A", 0, 0, string.Empty, string.Empty, string.Empty));
        Assert.Throws<ArgumentOutOfRangeException>(() => new AppliedEdit(-1, "A", 0, 0, string.Empty, string.Empty, null));
        Assert.Throws<ArgumentNullException>(() => new Referrer("A", null!));
        Assert.Throws<ArgumentNullException>(() => new EditResult(null, null!, [], []));
    }

    // An Edit each capability of the service's order gates, by a name the theory reports.
    private static Dictionary<string, (Edit Edit, string Capability)> Gated => new(StringComparer.Ordinal)
    {
        ["delete"] = (new Edit.Delete("A::B"), Capabilities.Authoring),
        ["move"] = (new Edit.Move("A::B", "A::C"), Capabilities.Authoring),
        ["member"] = (new Edit.AddMember("A", "part", "p"), Capabilities.Authoring),
        ["connection"] = (new Edit.AddConnection("A", "flow", "a", "b"), Capabilities.ConnectionAuthoring),
        ["satisfy"] = (new Edit.AddSatisfy("A", "R"), Capabilities.SatisfyAuthoring),
        ["requirement constraint"] = (new Edit.AddRequirementConstraint("A", "require", "x"), Capabilities.RequirementConstraintAuthoring),
        ["transition"] = (new Edit.AddTransition("A", "s", "t"), Capabilities.TransitionAuthoring),
        ["verify"] = (new Edit.AddVerify("A", "R"), Capabilities.VerificationObjectiveAuthoring),
        ["anonymous objective"] = (new Edit.AddMember("A", "objective", string.Empty), Capabilities.VerificationObjectiveAuthoring),
        ["metadata"] = (new Edit.AddMetadata("A", "M"), Capabilities.MetadataAuthoring),
        ["metadata prefixes"] = (new Edit.AddMember("A", "part", "p") { MetadataPrefixes = ["M"] }, Capabilities.MetadataAuthoring),
        ["metadata prefix"] = (new Edit.AddMetadataPrefix("A", "M"), Capabilities.MetadataPrefixAuthoring),
        ["sequence"] = (Edit.AddSequence.Then("A", "x"), Capabilities.SequenceAuthoring),
        ["condition"] = (Edit.AddSequence.GuardedThen("A", "c", "x"), Capabilities.ActionBodyStatementAuthoring),
        ["else"] = (Edit.AddSequence.Else("A", "x"), Capabilities.ActionBodyStatementAuthoring),
        ["value"] = (Edit.AddSequence.Then("A", "x") with { Value = "1" }, Capabilities.ActionBodyStatementAuthoring),
        ["target"] = (Edit.AddSequence.Then("A", "x") with { Target = "t" }, Capabilities.ActionBodyStatementAuthoring),
        ["via"] = (Edit.AddSequence.Then("A", "x") with { Via = "p" }, Capabilities.ActionBodyStatementAuthoring),
        ["until"] = (Edit.AddSequence.Then("A", "x") with { Until = "u" }, Capabilities.ActionBodyStatementAuthoring),
        ["multiplicity"] = (Edit.AddSequence.Then("A", "x") with { Multiplicity = "[1]" }, Capabilities.ActionBodyStatementAuthoring),
        ["parameter"] = (Edit.AddSequence.Then("A", "x") with { Parameter = "p" }, Capabilities.ActionBodyStatementAuthoring),
        ["else body"] = (Edit.AddSequence.Then("A", "x") with { ElseBody = [Edit.AddSequence.Then(string.Empty, "y")] }, Capabilities.ActionBodyStatementAuthoring),
        ["node"] = (Edit.AddSequence.Node("A", "action", "a"), Capabilities.ActionBodyStatementAuthoring),
        ["accept"] = (Edit.AddSequence.ThenMember("A", "accept", "a"), Capabilities.ActionBodyStatementAuthoring),
        ["terminate"] = (Edit.AddSequence.Terminate("A"), Capabilities.ActionBodyStatementAuthoring),
        ["nested condition"] = (
            Edit.AddSequence.ThenMember("A", "action", "a") with { Body = [Edit.AddSequence.Loop(string.Empty, [Edit.AddSequence.Then(string.Empty, "x")])] },
            Capabilities.ActionBodyStatementAuthoring),
        ["import"] = (new Edit.AddImport("A", "L::*"), Capabilities.ImportAuthoring),
        ["abstract"] = (new Edit.AddMember("A", "part def", "P") { IsAbstract = true }, Capabilities.MemberModifiers),
        ["redefines"] = (new Edit.AddMember("A", "part", "p") { Redefines = ["q"] }, Capabilities.MemberModifiers),
        ["default"] = (new Edit.AddMember("A", "attribute", "a") { IsDefault = true }, Capabilities.MemberModifiers),
        ["direction"] = (new Edit.AddMember("A", "attribute", "a") { Direction = "in" }, Capabilities.MemberModifiers),
        ["ref"] = (new Edit.AddMember("A", "ref", "r"), Capabilities.MemberModifiers),
        ["return"] = (new Edit.AddMember("A", "return", string.Empty), Capabilities.MemberModifiers),
        ["implicit"] = (new Edit.AddMember("A", string.Empty, "x"), Capabilities.ImplicitParameters),
        ["body expression"] = (new Edit.AddMember("A", "constraint", "c") { BodyExpression = "true" }, Capabilities.ConstraintBodyAuthoring),
        ["assert"] = (new Edit.AddMember("A", "assert", "c"), Capabilities.ConstraintBodyAuthoring),
        ["assert not constraint"] = (new Edit.AddMember("A", "assert not constraint", "c"), Capabilities.ConstraintBodyAuthoring),
        ["exhibit"] = (new Edit.AddMember("A", "exhibit state", "s"), Capabilities.StateActionAuthoring),
        ["entry action"] = (new Edit.AddMember("A", "entry action", "e"), Capabilities.StateActionAuthoring),
        ["documentation"] = (new Edit.AddDocumentation("A", "d"), Capabilities.DocumentationAuthoring),
        ["doc"] = (new Edit.AddMember("A", "part", "p") { Doc = "d" }, Capabilities.DocumentationAuthoring),
        ["comment"] = (new Edit.AddComment("A", "c"), Capabilities.CommentAuthoring),
        ["note"] = (new Edit.AddNote("A", "n"), Capabilities.CommentAuthoring),
    };

    private static FakeServiceHandler Serving(ClientWire wire) => new(wire) { Capabilities = FakeModels.ModelCapabilities };

    private static Edit.AddSequence Sequence(string property) => property switch
    {
        "Ref" => new Edit.AddSequence("A", "then") { Ref = string.Empty },
        "MemberKind" => new Edit.AddSequence("A", "then") { MemberKind = string.Empty },
        "MemberName" => new Edit.AddSequence("A", "then") { MemberName = string.Empty },
        "Type" => new Edit.AddSequence("A", "then") { Type = string.Empty },
        "After" => new Edit.AddSequence("A", "then") { After = string.Empty },
        "Condition" => new Edit.AddSequence("A", "then") { Condition = string.Empty },
        "Value" => new Edit.AddSequence("A", "then") { Value = string.Empty },
        "Target" => new Edit.AddSequence("A", "then") { Target = string.Empty },
        "Via" => new Edit.AddSequence("A", "then") { Via = string.Empty },
        "Until" => new Edit.AddSequence("A", "then") { Until = string.Empty },
        "Multiplicity" => new Edit.AddSequence("A", "then") { Multiplicity = string.Empty },
        _ => new Edit.AddSequence("A", "then") { Parameter = string.Empty },
    };

    // A request nesting deeper than the protobuf library reads by default, decoded as the service reads it.
    private static Wire.ApplyEditsRequest Deep(SentRequest sent)
    {
        if (sent.Wire == ClientWire.ConnectJson)
        {
            return new JsonParser(JsonParser.Settings.Default.WithRecursionLimit(1000)).Parse<Wire.ApplyEditsRequest>(Encoding.UTF8.GetString(sent.Body));
        }

        return Wire.ApplyEditsRequest.Parser.ParseFrom(CodedInputStream.CreateWithLimits(new MemoryStream(sent.MessageBytes), int.MaxValue, 1000));
    }

    private static Diagnostic ModelWireDiagnostic() => Internal.ModelWire.Diagnostic(CompleteWire.Diagnostic);

    // The capability an edit is refused for when the service lacks the given ones, after checking nothing was sent.
    private static async Task<string> RefusedAsync(ClientWire wire, string[] withheld, Func<Model, Task> call)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except(withheld)] };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var refused = await Assert.ThrowsAsync<CapabilityException>(() => call(connection.ModelByHash(Hash)));

        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        return refused.Capability;
    }
}
