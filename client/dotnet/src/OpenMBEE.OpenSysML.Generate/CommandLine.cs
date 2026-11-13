using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML.Generate;

/// <summary>What the command was asked to do.</summary>
/// <param name="Sources">The model's files and directories, in order.</param>
/// <param name="Output">The file to write or check; null to write to standard output.</param>
/// <param name="Namespace">The C# namespace the generated types are declared under.</param>
/// <param name="Check">Whether to compare with <paramref name="Output"/> rather than write it.</param>
/// <param name="Address">The service to connect to; null for the default a connection chooses.</param>
/// <param name="Binary">The <c>sysml-grpc</c> binary to start a private service from; null for the default.</param>
/// <param name="Help">Whether only the usage was asked for.</param>
internal sealed record CommandLine(
    IReadOnlyList<string> Sources,
    string? Output,
    string Namespace,
    bool Check,
    string? Address,
    string? Binary,
    bool Help)
{
    /// <summary>The usage the command prints for <c>--help</c>.</summary>
    internal const string Usage = """
        usage: OpenMBEE.OpenSysML.Generate SOURCE... --namespace NAMESPACE [-o OUTPUT] [--check]
                                          [--address ADDRESS | --binary PATH]

        Generate typed C# interfaces and classes from a SysML model. The model is one or more .sysml or
        .kerml files, parsed as one model in the order given; a directory contributes every model file
        beneath it in path order.

        arguments:
          SOURCE                 a .sysml or .kerml file, or a directory of them

        options:
          --namespace NAMESPACE  the C# namespace to declare the generated types under (required)
          -o, --output OUTPUT    the .cs file to write (default: standard output)
          --check                write nothing; fail if regenerating would change OUTPUT
          --address ADDRESS      a running sysml-grpc service to use, as host:port or a URL
                                 (default: $OPENSYSML_SERVICE, else a private service)
          --binary PATH          the sysml-grpc binary to start a private service from
                                 (default: $OPENSYSML_GRPC_BINARY, else the release this client pins)
          -h, --help             show this message and exit
        """;

    /// <summary>Reads the arguments.</summary>
    /// <exception cref="UsageException">An option is unknown or lacks its value, or a required one is missing.</exception>
    internal static CommandLine Parse(IReadOnlyList<string> arguments)
    {
        var sources = new List<string>();
        string? output = null;
        string? @namespace = null;
        string? address = null;
        string? binary = null;
        var check = false;
        var optionsEnded = false;
        for (var i = 0; i < arguments.Count; i++)
        {
            var argument = arguments[i];
            if (optionsEnded || !argument.StartsWith('-') || argument == "-")
            {
                sources.Add(argument);
                continue;
            }

            var (name, inline) = argument.IndexOf('=', StringComparison.Ordinal) is var at and > 0 && argument.StartsWith("--", StringComparison.Ordinal)
                ? (argument[..at], argument[(at + 1)..])
                : (argument, null);
            switch (name)
            {
                case "--":
                    optionsEnded = true;
                    break;
                case "-h" or "--help":
                    return new CommandLine([], null, string.Empty, false, null, null, Help: true);
                case "--check" when inline is null:
                    check = true;
                    break;
                case "-o" or "--output":
                    output = Value(arguments, ref i, name, inline);
                    break;
                case "--namespace":
                    @namespace = Value(arguments, ref i, name, inline);
                    break;
                case "--address":
                    address = Value(arguments, ref i, name, inline);
                    break;
                case "--binary":
                    binary = Value(arguments, ref i, name, inline);
                    break;
                default:
                    throw new UsageException($"unknown option {argument}");
            }
        }

        if (sources.Count == 0)
        {
            throw new UsageException("name at least one SOURCE");
        }

        if (@namespace is null)
        {
            throw new UsageException("--namespace is required");
        }

        if (check && output is null)
        {
            throw new UsageException("--check needs -o/--output, the file to check");
        }

        if (address is not null && binary is not null)
        {
            throw new UsageException("--address names a running service and --binary one to start; give one");
        }

        foreach (var segment in @namespace.Split('.'))
        {
            if (!Identifiers.IsPlainIdentifier(segment) || segment == "global")
            {
                throw new UsageException($"--namespace {@namespace} is not a C# namespace: '{segment}' is not an identifier it can hold");
            }
        }

        return new CommandLine(sources, output, @namespace, check, address, binary, Help: false);
    }

    /// <summary>The command a <c>--check</c> failure tells its reader to regenerate with.</summary>
    internal string RegenerateCommand()
    {
        var words = new List<string> { "dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Generate --" };
        words.AddRange(Sources);
        words.Add($"-o {Output}");
        words.Add($"--namespace {Namespace}");
        if (Address is not null)
        {
            words.Add($"--address {Address}");
        }

        if (Binary is not null)
        {
            words.Add($"--binary {Binary}");
        }

        return string.Join(' ', words);
    }

    private static string Value(IReadOnlyList<string> arguments, ref int index, string name, string? inline)
    {
        if (inline is not null)
        {
            return inline.Length > 0 ? inline : throw new UsageException($"{name} is missing its value");
        }

        if (index + 1 >= arguments.Count || arguments[index + 1].Length == 0)
        {
            throw new UsageException($"{name} is missing its value");
        }

        return arguments[++index];
    }
}
