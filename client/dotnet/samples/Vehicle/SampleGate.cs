using System;
using System.Reflection;
using System.Runtime.Versioning;
using OpenMBEE.OpenSysML;

/// <summary>What the sample prints before its walkthrough, which the build's sample gate reads.</summary>
internal static class SampleGate
{
    private const string ClientAssembly = "OpenMBEE.OpenSysML";

    /// <summary>
    /// Prints the referenced client assembly's version and target framework. Answers the exit code
    /// when the sample must stop here: 1 when the assembly carries neither, 0 when neither
    /// OPENSYSML_SERVICE nor OPENSYSML_GRPC_BINARY names a service to reach; otherwise null.
    /// </summary>
    internal static int? Check()
    {
        var client = typeof(Connection).Assembly;
        var version = client.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion;
        var framework = client.GetCustomAttribute<TargetFrameworkAttribute>()?.FrameworkName;
        if (version is null || framework is null)
        {
            Console.Error.WriteLine($"{ClientAssembly} carries no informational version or target framework.");
            return 1;
        }

        Console.WriteLine($"{ClientAssembly} {version} ({framework})");
        if (string.IsNullOrWhiteSpace(Environment.GetEnvironmentVariable(ConnectionOptions.ServiceVariable))
            && string.IsNullOrWhiteSpace(Environment.GetEnvironmentVariable(ConnectionOptions.BinaryVariable)))
        {
            return 0;
        }

        return null;
    }
}
