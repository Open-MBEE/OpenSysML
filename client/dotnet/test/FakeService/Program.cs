using System;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.Json;
using System.Threading;

// Reads what to do from fake-service.json in the directory of the launcher that was started, so
// a test chooses the behavior by where it puts a copy, with the argv the client gives every
// private service. Nothing it does depends on that argv beyond recording it.
var directory = Path.GetDirectoryName(Environment.ProcessPath) ?? AppContext.BaseDirectory;
var script = JsonSerializer.Deserialize<Script>(
    File.ReadAllText(Path.Combine(directory, "fake-service.json")),
    new JsonSerializerOptions { PropertyNamingPolicy = JsonNamingPolicy.CamelCase }) ?? new Script();

if (script.ArgumentsFile is { } argumentsFile)
{
    File.WriteAllLines(argumentsFile, args);
}

if (script.ProcessIdFile is { } processIdFile)
{
    File.WriteAllText(processIdFile, Environment.ProcessId.ToString(System.Globalization.CultureInfo.InvariantCulture));
}

using var stdinClosed = new ManualResetEventSlim();
var stdinReader = new Thread(() =>
{
    using var input = Console.OpenStandardInput();
    var buffer = new byte[256];
    while (input.Read(buffer) > 0)
    {
    }

    stdinClosed.Set();
})
{ IsBackground = true };
stdinReader.Start();

var stdout = Console.OpenStandardOutput();
var stderr = Console.OpenStandardError();
if (script.Stderr is { } said)
{
    Write(stderr, said + "\n");
}

Flood(stderr, script.StderrBytesBeforeReport, script.FloodLineLength);
if (script.DelayMilliseconds > 0)
{
    Thread.Sleep(script.DelayMilliseconds);
}

if (script.Report is { } report)
{
    Write(stdout, script.ReportWithoutNewline ? report : report + "\n");
}

if (script.CloseStdout)
{
    // The console stream and the runtime each hold a duplicate of the descriptor; the reader sees end of
    // file only once every one is closed.
    stdout.Close();
    Native.CloseStandardOutput();
}

if (script.ExitCode is { } code)
{
    return code;
}

if (script.FloodAfterReport)
{
    foreach (var stream in new[] { stdout, stderr })
    {
        new Thread(() =>
        {
            while (!stdinClosed.IsSet)
            {
                Flood(stream, 64 * 1024, script.FloodLineLength);
            }
        })
        { IsBackground = true }.Start();
    }
}

if (script.IgnoreStdinEnd)
{
    Thread.Sleep(Timeout.Infinite);
}

stdinClosed.Wait();
if (script.ExitMarker is { } marker)
{
    File.WriteAllText(marker, "stdin closed\n");
}

return 0;

static void Write(Stream stream, string text)
{
    try
    {
        stream.Write(Encoding.UTF8.GetBytes(text));
        stream.Flush();
    }
    catch (IOException)
    {
        // The reader went away; a stand-in has no one to tell.
    }
}

static void Flood(Stream stream, int bytes, int lineLength)
{
    if (bytes <= 0)
    {
        return;
    }

    var text = new StringBuilder(bytes);
    for (var i = 0; i < bytes; i++)
    {
        text.Append(lineLength > 0 && i % lineLength == lineLength - 1 ? '\n' : 'x');
    }

    Write(stream, text.ToString());
}

/// <summary>What the stand-in does, in order.</summary>
internal sealed class Script
{
    /// <summary>A file to write the arguments to, one per line.</summary>
    public string? ArgumentsFile { get; init; }

    /// <summary>A file to write the process id to.</summary>
    public string? ProcessIdFile { get; init; }

    /// <summary>A line written to stderr first.</summary>
    public string? Stderr { get; init; }

    /// <summary>How many bytes to write to stderr before the address.</summary>
    public int StderrBytesBeforeReport { get; init; }

    /// <summary>How long a flooded line is, newline included; zero for one line without end.</summary>
    public int FloodLineLength { get; init; } = 100;

    /// <summary>How long to wait before the address.</summary>
    public int DelayMilliseconds { get; init; }

    /// <summary>The first line of stdout; null to report nothing.</summary>
    public string? Report { get; init; }

    /// <summary>Whether the report is written without ending its line.</summary>
    public bool ReportWithoutNewline { get; init; }

    /// <summary>Whether stdout is closed after the report, the process running on.</summary>
    public bool CloseStdout { get; init; }

    /// <summary>The code to exit with after the report, rather than waiting for stdin to end.</summary>
    public int? ExitCode { get; init; }

    /// <summary>Whether both streams are flooded after the report until stdin ends.</summary>
    public bool FloodAfterReport { get; init; }

    /// <summary>Whether the end of stdin is ignored, so only a kill ends the process.</summary>
    public bool IgnoreStdinEnd { get; init; }

    /// <summary>A file written when stdin ends, proving an orderly exit.</summary>
    public string? ExitMarker { get; init; }
}

/// <summary>Closes the process's own standard output, of which the console streams and the runtime hold duplicates.</summary>
internal static partial class Native
{
    private const int StandardOutputHandle = -11;

    // Device and inode lead struct stat on every platform this runs on, whatever their widths.
    private const int IdentityBytes = 16;

    /// <summary>Closes descriptor 1 and every duplicate of it, or the standard output handle on Windows.</summary>
    internal static void CloseStandardOutput()
    {
        if (OperatingSystem.IsWindows())
        {
            CloseHandle(GetStdHandle(StandardOutputHandle));
            return;
        }

        var output = new byte[1024];
        if (FStat(1, output) == 0)
        {
            var other = new byte[1024];
            for (var descriptor = 3; descriptor < 1024; descriptor++)
            {
                if (FStat(descriptor, other) == 0 && output.AsSpan(0, IdentityBytes).SequenceEqual(other.AsSpan(0, IdentityBytes)))
                {
                    _ = Close(descriptor);
                }
            }
        }

        _ = Close(1);
    }

    [LibraryImport("libc", EntryPoint = "close")]
    private static partial int Close(int descriptor);

    [LibraryImport("libc", EntryPoint = "fstat")]
    private static partial int FStat(int descriptor, [Out] byte[] status);

    [LibraryImport("kernel32.dll")]
    private static partial IntPtr GetStdHandle(int handle);

    [LibraryImport("kernel32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static partial bool CloseHandle(IntPtr handle);
}
