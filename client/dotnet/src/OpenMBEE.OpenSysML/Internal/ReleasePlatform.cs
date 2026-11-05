using System;
using System.Collections.Frozen;
using System.Linq;
using System.Runtime.InteropServices;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The release asset of <c>sysml-grpc</c> built for a platform, named as the release pipeline names it, by Go's
/// <c>GOOS</c> and <c>GOARCH</c>: <c>sysml-grpc-linux-amd64</c>, <c>sysml-grpc-windows-amd64.exe</c>. A pair no
/// release publishes a binary for is refused by name rather than fetched as a 404.
/// </summary>
internal static class ReleasePlatform
{
    /// <summary>Every <c>goos-goarch</c> a release publishes a <c>sysml-grpc</c> binary for.</summary>
    internal static readonly FrozenSet<string> Published =
        FrozenSet.Create(StringComparer.Ordinal, "linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64");

    /// <summary>Go's name for this operating system.</summary>
    internal static string HostGoos =>
        OperatingSystem.IsLinux() ? "linux"
        : OperatingSystem.IsMacOS() ? "darwin"
        : OperatingSystem.IsWindows() ? "windows"
        : OperatingSystem.IsFreeBSD() ? "freebsd"
        : OperatingSystem.IsAndroid() ? "android"
        : OperatingSystem.IsIOS() ? "ios"
        : OperatingSystem.IsBrowser() ? "js"
        : OperatingSystem.IsWasi() ? "wasip1"
        : "unknown";

    /// <summary>Go's name for this machine's architecture.</summary>
    internal static string HostGoarch => Goarch(RuntimeInformation.OSArchitecture);

    /// <summary>The asset a release publishes the binary for this machine under.</summary>
    /// <returns>The asset name.</returns>
    /// <exception cref="UnsupportedPlatformException">No release is built for this machine.</exception>
    internal static string HostAsset() => Asset(HostGoos, HostGoarch);

    /// <summary>The asset a release publishes the binary for a platform under.</summary>
    /// <param name="goos">Go's name for the operating system.</param>
    /// <param name="goarch">Go's name for the architecture.</param>
    /// <returns>The asset name, with <c>.exe</c> for Windows.</returns>
    /// <exception cref="UnsupportedPlatformException">No release is built for that pair.</exception>
    internal static string Asset(string goos, string goarch)
    {
        var pair = $"{goos}-{goarch}";
        if (!Published.Contains(pair))
        {
            throw new UnsupportedPlatformException(
                pair,
                $"No sysml-grpc release is published for {pair}; releases are built for {string.Join(", ", Published.Order(StringComparer.Ordinal))}.\n"
                + $"  fix: build one (make build-grpc) and name it with ConnectionOptions.BinaryPath or ${ConnectionOptions.BinaryVariable}, "
                + $"or name a running service with ConnectionOptions.Address or ${ConnectionOptions.ServiceVariable}.");
        }

        return goos == "windows" ? $"sysml-grpc-{pair}.exe" : $"sysml-grpc-{pair}";
    }

    /// <summary>Go's name for an architecture.</summary>
    /// <param name="architecture">The architecture.</param>
    /// <returns>Its <c>GOARCH</c>.</returns>
    internal static string Goarch(Architecture architecture) => architecture switch
    {
        Architecture.X64 => "amd64",
        Architecture.Arm64 => "arm64",
        Architecture.X86 => "386",
        Architecture.Arm or Architecture.Armv6 => "arm",
        Architecture.Wasm => "wasm",
        Architecture.S390x => "s390x",
        Architecture.LoongArch64 => "loong64",
        Architecture.Ppc64le => "ppc64le",
        Architecture.RiscV64 => "riscv64",
        _ => architecture.ToString().ToLowerInvariant(),
    };
}
