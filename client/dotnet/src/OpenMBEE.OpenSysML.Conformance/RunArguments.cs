using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>What a conformance run was asked to do, as its command line states it.</summary>
internal sealed record RunOptions
{
    /// <summary>The protocols a run tests unless <c>-protocols</c> names others.</summary>
    internal static readonly IReadOnlyList<string> DefaultProtocols = ["grpc", "connect", "connect-json"];

    /// <summary>The conformance directory, holding scenarios/ and fixtures/.</summary>
    internal required string Directory { get; init; }

    /// <summary>The expected-skips list.</summary>
    internal required string ExpectedSkips { get; init; }

    /// <summary>The sysml-grpc the run starts, once per configuration; null when <see cref="Service"/> names one.</summary>
    internal string? Binary { get; init; }

    /// <summary>The address of a service already running; null when <see cref="Binary"/> names one to start.</summary>
    internal string? Service { get; init; }

    /// <summary>The transport a started service serves: connect or grpc.</summary>
    internal string Transport { get; init; } = "connect";

    /// <summary>The protocols to test, in order.</summary>
    internal IReadOnlyList<string> Protocols { get; init; } = DefaultProtocols;

    /// <summary>The capabilities a second configuration withholds; empty for none.</summary>
    internal IReadOnlyList<string> Withhold { get; init; } = [];

    /// <summary>Selects scenarios by id; null runs them all.</summary>
    internal Regex? Filter { get; init; }

    /// <summary>Where the JSON report goes: a file, <c>-</c> for stdout, or null for nowhere.</summary>
    internal string? Report { get; init; }

    /// <summary>Where the JUnit report goes, or null for nowhere.</summary>
    internal string? JUnit { get; init; }

    /// <summary>Whether every skip is accepted.</summary>
    internal bool AllowSkips { get; init; }

    /// <summary>The corruption applied to every answer.</summary>
    internal Mutation Mutation { get; init; }

    /// <summary>Whether to print each normalized response.</summary>
    internal bool Verbose { get; init; }
}

/// <summary>A command line the runner cannot act on; it exits 2 with the usage.</summary>
/// <param name="message">What is wrong with it.</param>
internal sealed class UsageException(string message) : Exception(message);

/// <summary>Reads the runner's command line. Every flag is single-dash, as the reference runner's are.</summary>
internal static class RunArguments
{
    /// <summary>The usage the runner prints with a usage error or <c>-h</c>.</summary>
    internal static readonly string Usage = $"""
        usage: OpenMBEE.OpenSysML.Conformance (-binary <path> | -service <host:port>) [options]

          -binary <path>        sysml-grpc to start, once per configuration
          -service <host:port>  a service already running, instead of one to start
          -dir <path>           conformance directory (default: the checkout's conformance/)
          -expected-skips <path> the reviewed skips (default: the checkout's client/dotnet/test/testdata/expected-skips.json)
          -run <regexp>         only the scenarios whose id matches
          -protocols <list>     comma-separated, of grpc, connect, connect-json (default: all three)
          -transport <name>     what a started service serves: connect (default) or grpc
          -withhold-capabilities <list>  also test a started service with these capabilities withheld
          -report <path>        write the JSON report here (- for stdout)
          -junit <path>         also write the results as JUnit XML here
          -allow-skips          accept every skip, not only the expected ones
          -mutate <name>        corrupt every answer: {string.Join(", ", Mutations.Names)}
          -v                    print each normalized response
          -h                    print this usage

        Values may also use -flag=value; use this form for a value starting with - (except - alone).

        """;

    /// <summary>Reads a command line.</summary>
    /// <param name="args">The arguments.</param>
    /// <param name="workingDirectory">Where to look upwards for the checkout the defaults name.</param>
    /// <returns>The options, or null when <c>-h</c> asked only for the usage.</returns>
    /// <exception cref="UsageException">The command line is malformed, incomplete or contradictory.</exception>
    internal static RunOptions? Parse(IReadOnlyList<string> args, string workingDirectory)
    {
        ArgumentNullException.ThrowIfNull(args);
        string? directory = null, expectedSkips = null, binary = null, service = null, transport = null;
        string? report = null, junit = null;
        List<string>? protocols = null;
        List<string> withhold = [];
        Regex? filter = null;
        var mutation = Mutation.None;
        bool allowSkips = false, verbose = false;
        var seen = new HashSet<string>(StringComparer.Ordinal);

        for (var i = 0; i < args.Count; i++)
        {
            var argument = args[i];
            var separator = argument.IndexOf('=');
            var flag = separator < 0 ? argument : argument[..separator];
            if (!seen.Add(flag))
            {
                throw new UsageException($"{flag} is given more than once");
            }

            if (separator >= 0 && flag is "-h" or "-help" or "--help" or "-allow-skips" or "-v")
            {
                throw new UsageException($"{flag} does not take a value");
            }

            switch (flag)
            {
                case "-h" or "-help" or "--help":
                    return null;
                case "-dir":
                    directory = Value(args, ref i);
                    break;
                case "-expected-skips":
                    expectedSkips = Value(args, ref i);
                    break;
                case "-binary":
                    binary = Value(args, ref i);
                    break;
                case "-service":
                    service = Value(args, ref i);
                    break;
                case "-transport":
                    transport = Value(args, ref i);
                    break;
                case "-run":
                    filter = Pattern(Value(args, ref i));
                    break;
                case "-protocols":
                    protocols = Protocols(Value(args, ref i));
                    break;
                case "-withhold-capabilities":
                    withhold = Names(Value(args, ref i), "capability");
                    break;
                case "-report":
                    report = Value(args, ref i);
                    break;
                case "-junit":
                    junit = Value(args, ref i);
                    break;
                case "-mutate":
                    var name = Value(args, ref i);
                    if (!Mutations.TryParse(name, out mutation))
                    {
                        throw new UsageException($"unknown -mutate {Spelling.Quote(name)}; want one of {string.Join(", ", Mutations.Names)}");
                    }

                    break;
                case "-allow-skips":
                    allowSkips = true;
                    break;
                case "-v":
                    verbose = true;
                    break;
                default:
                    throw new UsageException(flag.StartsWith('-') ? $"unknown flag {flag}" : $"unexpected argument {Spelling.Quote(argument)}");
            }
        }

        if ((binary is null) == (service is null))
        {
            throw new UsageException("name the service to test: -binary to start one, or -service for one already running");
        }

        if (service is not null && transport is not null)
        {
            throw new UsageException("-transport chooses what a started service serves; it does not apply to -service");
        }

        if (service is not null && withhold.Count > 0)
        {
            throw new UsageException("-withhold-capabilities needs a service the run starts; it does not apply to -service");
        }

        transport ??= "connect";
        if (transport is not ("connect" or "grpc"))
        {
            throw new UsageException($"unknown -transport {Spelling.Quote(transport)}; want connect or grpc");
        }

        IReadOnlyList<string> tested = protocols ?? RunOptions.DefaultProtocols;
        if (transport == "grpc" && tested.FirstOrDefault(protocol => protocol != "grpc") is { } other)
        {
            throw new UsageException($"protocol {other} needs -transport connect; a grpc-only service speaks grpc alone");
        }

        var checkout = directory is null || expectedSkips is null ? Checkout(workingDirectory) : null;
        return new RunOptions
        {
            Directory = Path.GetFullPath(directory ?? Path.Combine(checkout!, "conformance"), workingDirectory),
            ExpectedSkips = Path.GetFullPath(expectedSkips ?? Path.Combine(checkout!, "client", "dotnet", "test", "testdata", "expected-skips.json"), workingDirectory),
            Binary = binary is null ? null : Path.GetFullPath(binary, workingDirectory),
            Service = service,
            Transport = transport,
            Protocols = tested,
            Withhold = withhold,
            Filter = filter,
            Report = report is null or "-" ? report : Path.GetFullPath(report, workingDirectory),
            JUnit = junit is null ? null : Path.GetFullPath(junit, workingDirectory),
            AllowSkips = allowSkips,
            Mutation = mutation,
            Verbose = verbose,
        };
    }

    // The checkout above a directory: the nearest holding conformance/scenarios.
    private static string Checkout(string start)
    {
        for (var directory = new DirectoryInfo(start); directory is not null; directory = directory.Parent)
        {
            if (System.IO.Directory.Exists(Path.Combine(directory.FullName, "conformance", "scenarios")))
            {
                return directory.FullName;
            }
        }

        throw new UsageException($"no checkout holding conformance/scenarios above {start}; name -dir and -expected-skips");
    }

    private static string Value(IReadOnlyList<string> args, ref int i)
    {
        var flag = args[i];
        var separator = flag.IndexOf('=');
        if (separator >= 0)
        {
            return separator + 1 < flag.Length
                ? flag[(separator + 1)..]
                : throw new UsageException($"{flag[..separator]} wants a value");
        }

        if (++i >= args.Count || args[i].Length == 0 || (args[i].StartsWith('-') && args[i] != "-"))
        {
            throw new UsageException($"{flag} wants a value");
        }

        return args[i];
    }

    private static Regex Pattern(string pattern)
    {
        try
        {
            return new Regex(pattern, RegexOptions.CultureInvariant, TimeSpan.FromSeconds(1));
        }
        catch (ArgumentException e)
        {
            throw new UsageException($"bad -run pattern {Spelling.Quote(pattern)}: {e.Message}");
        }
    }

    private static List<string> Protocols(string value)
    {
        var protocols = Names(value, "protocol");
        if (protocols.Count == 0)
        {
            throw new UsageException("-protocols must name at least one protocol");
        }

        if (protocols.FirstOrDefault(protocol => !RunOptions.DefaultProtocols.Contains(protocol)) is { } unknown)
        {
            throw new UsageException($"unknown protocol {Spelling.Quote(unknown)}; want grpc, connect or connect-json");
        }

        return protocols;
    }

    private static List<string> Names(string value, string what)
    {
        var names = new List<string>();
        foreach (var item in value.Split(',').Select(item => item.Trim()).Where(item => item.Length > 0))
        {
            if (names.Contains(item, StringComparer.Ordinal))
            {
                throw new UsageException($"{what} {Spelling.Quote(item)} is listed more than once");
            }

            names.Add(item);
        }

        return names;
    }
}
