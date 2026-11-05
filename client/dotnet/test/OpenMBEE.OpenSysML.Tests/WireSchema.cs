using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf.Reflection;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The schema api/proto/sysml.proto declares, as buf builds it, set beside the descriptor compiled into the committed
/// protocol code, so that code cannot drift from the schema it was generated from unnoticed.
/// </summary>
internal static class WireSchema
{
    /// <summary>What to run when the two differ.</summary>
    internal const string Remedy = "Run make proto-csharp and commit client/dotnet/src/OpenMBEE.OpenSysML/Wire.";

    // A cold `go run` of buf compiles it first.
    private static readonly TimeSpan Deadline = TimeSpan.FromMinutes(5);

    /// <summary>Builds the schema with buf and returns its sysml.proto.</summary>
    /// <param name="command">buf's program and the arguments before its own, as <see cref="BufGate"/> found them.</param>
    internal static async Task<FileDescriptorProto> BuildAsync(IReadOnlyList<string> command)
    {
        var start = new ProcessStartInfo(command[0])
        {
            WorkingDirectory = Repository.Root,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        foreach (var argument in command.Skip(1).Concat(["build", Repository.PathOf("api", "proto"), "--as-file-descriptor-set", "-o", "-#format=binpb"]))
        {
            start.ArgumentList.Add(argument);
        }

        var described = string.Join(' ', start.ArgumentList.Prepend(start.FileName));
        using var process = Process.Start(start) ?? throw new InvalidOperationException($"{described} did not start.");
        using var output = new MemoryStream();
        var copied = process.StandardOutput.BaseStream.CopyToAsync(output, TestContext.Current.CancellationToken);
        var errors = process.StandardError.ReadToEndAsync(TestContext.Current.CancellationToken);
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(TestContext.Current.CancellationToken);
        deadline.CancelAfter(Deadline);
        try
        {
            await process.WaitForExitAsync(deadline.Token);
        }
        catch (OperationCanceledException)
        {
            process.Kill(entireProcessTree: true);
            await process.WaitForExitAsync(CancellationToken.None);
            TestContext.Current.CancellationToken.ThrowIfCancellationRequested();
            Assert.Fail($"{described} did not finish within {Deadline.TotalMinutes} minutes.");
        }

        await copied;
        var said = (await errors).Trim();
        Assert.True(process.ExitCode == 0, $"{described} exited with code {process.ExitCode}: {said}");

        var name = Wire.SysmlReflection.Descriptor.Name;
        var files = FileDescriptorSet.Parser.ParseFrom(output.ToArray()).File;
        return files.SingleOrDefault(file => file.Name == name)
            ?? throw new InvalidOperationException($"{described} built no {name}, only {string.Join(", ", files.Select(file => file.Name))}.");
    }

    /// <summary>
    /// Where the schema and the committed code first part, in declaration order, or null when they agree on every
    /// message, field, oneof, reserved number and name, enum value and RPC. Options and source information are not
    /// compared: managed mode rewrites the C# namespace in what only the C# plugins see.
    /// </summary>
    /// <param name="schema">The schema buf builds.</param>
    /// <param name="committed">The descriptor compiled into the committed code.</param>
    internal static string? FirstDifference(FileDescriptorProto schema, FileDescriptorProto committed)
    {
        var expected = Outline(schema);
        var actual = Outline(committed);
        for (var i = 0; i < Math.Max(expected.Count, actual.Count); i++)
        {
            var want = i < expected.Count ? expected[i] : null;
            var have = i < actual.Count ? actual[i] : null;
            if (!string.Equals(want, have, StringComparison.Ordinal))
            {
                return $"api/proto/{schema.Name} and the committed C# protocol code first differ at declaration {i + 1}: "
                    + $"the schema has {want ?? "nothing more"}, where the code has {have ?? "nothing more"}. {Remedy}";
            }
        }

        return null;
    }

    /// <summary>Every declaration of <paramref name="file"/> the generated code depends on, one line each, in declaration order.</summary>
    internal static IReadOnlyList<string> Outline(FileDescriptorProto file)
    {
        var lines = new List<string> { $"file {file.Name}", $"syntax {file.Syntax}", $"package {file.Package}" };
        lines.AddRange(file.Dependency.Select(dependency => $"import {dependency}"));
        var scope = file.Package.Length == 0 ? string.Empty : file.Package + ".";
        foreach (var message in file.MessageType)
        {
            Message(message, scope, lines);
        }

        foreach (var declared in file.EnumType)
        {
            Enum(declared, scope, lines);
        }

        foreach (var service in file.Service)
        {
            var name = scope + service.Name;
            lines.Add($"service {name}");
            lines.AddRange(service.Method.Select(method =>
                $"rpc {name}.{method.Name}({(method.ClientStreaming ? "stream " : string.Empty)}{method.InputType}) "
                + $"returns ({(method.ServerStreaming ? "stream " : string.Empty)}{method.OutputType})"));
        }

        return lines;
    }

    private static void Message(DescriptorProto message, string scope, List<string> lines)
    {
        var name = scope + message.Name;
        lines.Add($"message {name}");
        lines.AddRange(message.OneofDecl.Select(oneof => $"oneof {name}.{oneof.Name}"));
        foreach (var field in message.Field)
        {
            var type = field.HasTypeName ? $"{field.Type} {field.TypeName}" : field.Type.ToString();
            var oneof = field.HasOneofIndex ? $" in oneof {message.OneofDecl[field.OneofIndex].Name}" : string.Empty;
            var optional = field.Proto3Optional ? " proto3_optional" : string.Empty;
            lines.Add($"field {name}.{field.Name} = {field.Number}, {field.Label} {type}{oneof}{optional}");
        }

        lines.AddRange(message.ReservedRange.Select(range => $"reserved {name} {range.Start} to {range.End - 1}"));
        lines.AddRange(message.ReservedName.Select(reserved => $"reserved {name} \"{reserved}\""));
        foreach (var nested in message.NestedType)
        {
            Message(nested, name + ".", lines);
        }

        foreach (var nested in message.EnumType)
        {
            Enum(nested, name + ".", lines);
        }
    }

    private static void Enum(EnumDescriptorProto declared, string scope, List<string> lines)
    {
        var name = scope + declared.Name;
        lines.Add($"enum {name}");
        lines.AddRange(declared.Value.Select(value => $"value {name}.{value.Name} = {value.Number}"));
        lines.AddRange(declared.ReservedRange.Select(range => $"reserved {name} {range.Start} to {range.End}"));
        lines.AddRange(declared.ReservedName.Select(reserved => $"reserved {name} \"{reserved}\""));
    }
}
