using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What one external tool call would be given, composed without starting the process: the preview
/// <c>sysml -tool-dry-run</c> prints, which <see cref="AnalysisOptions.ToolDryRun"/> asks for.
/// </summary>
public sealed record ToolDryRun
{
    /// <summary>Creates a preview, copying its collections.</summary>
    /// <param name="action">The <c>ToolExecution</c>-annotated action the call was made for.</param>
    /// <param name="tool">The tool the manifest entry names.</param>
    /// <param name="version">The version its manifest declares.</param>
    /// <param name="manifest">The manifest file.</param>
    /// <param name="executable">The executable it resolved to.</param>
    /// <param name="protocol">The protocol it is spoken under.</param>
    /// <param name="args">The argv entries after the executable.</param>
    /// <param name="environment">The environment, in <c>NAME=value</c> pairs, sorted.</param>
    /// <param name="workingDirectory">The working directory; empty inherits the service's.</param>
    /// <param name="stdin">What standard input carries: <c>json</c>, <c>none</c>, <c>csv</c> or <c>template</c>.</param>
    /// <param name="stdinData">The bytes standard input would carry.</param>
    /// <param name="inputFile">The file the invocation writes; null when it writes none.</param>
    /// <param name="outputDirectory">Whether the entry uses <c>{outputDir}</c>.</param>
    /// <param name="inputs">The values the call sends.</param>
    /// <param name="outputs">The bindings it expects back.</param>
    /// <param name="replySource">Where the reply is read: <c>stdout</c>, or <c>file:&lt;template&gt;</c>.</param>
    /// <param name="lines">The preview as the command prints it, one line each.</param>
    /// <exception cref="ArgumentNullException">An argument other than <paramref name="inputFile"/> is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public ToolDryRun(
        string action,
        string tool,
        string version,
        string manifest,
        string executable,
        string protocol,
        IEnumerable<string> args,
        IEnumerable<string> environment,
        string workingDirectory,
        string stdin,
        string stdinData,
        ToolDryRunFile? inputFile,
        bool outputDirectory,
        IEnumerable<ToolDryRunInput> inputs,
        IEnumerable<ToolDryRunOutput> outputs,
        string replySource,
        IEnumerable<string> lines)
    {
        ArgumentNullException.ThrowIfNull(action);
        ArgumentNullException.ThrowIfNull(tool);
        ArgumentNullException.ThrowIfNull(version);
        ArgumentNullException.ThrowIfNull(manifest);
        ArgumentNullException.ThrowIfNull(executable);
        ArgumentNullException.ThrowIfNull(protocol);
        ArgumentNullException.ThrowIfNull(workingDirectory);
        ArgumentNullException.ThrowIfNull(stdin);
        ArgumentNullException.ThrowIfNull(stdinData);
        ArgumentNullException.ThrowIfNull(replySource);
        Action = action;
        Tool = tool;
        Version = version;
        Manifest = manifest;
        Executable = executable;
        Protocol = protocol;
        Args = new ValueList<string>(args);
        Environment = new ValueList<string>(environment);
        WorkingDirectory = workingDirectory;
        Stdin = stdin;
        StdinData = stdinData;
        InputFile = inputFile;
        OutputDirectory = outputDirectory;
        Inputs = new ValueList<ToolDryRunInput>(inputs);
        Outputs = new ValueList<ToolDryRunOutput>(outputs);
        ReplySource = replySource;
        Lines = new ValueList<string>(lines);
    }

    /// <summary>The <c>ToolExecution</c>-annotated action the call was made for.</summary>
    public string Action { get; }

    /// <summary>The tool the manifest entry names.</summary>
    public string Tool { get; }

    /// <summary>The version the tool's manifest declares.</summary>
    public string Version { get; }

    /// <summary>The manifest file the entry was read from.</summary>
    public string Manifest { get; }

    /// <summary>The executable the tool resolved to.</summary>
    public string Executable { get; }

    /// <summary>The protocol the tool is spoken under.</summary>
    public string Protocol { get; }

    /// <summary>The argv entries after the executable, as the invocation block renders them; empty for an entry without one.</summary>
    public ValueList<string> Args { get; }

    /// <summary>
    /// The environment in <c>NAME=value</c> pairs, sorted; a variable taken from the service's own process is spelled
    /// <c>NAME=&lt;from this process&gt;</c>. Empty when the entry inherits the whole environment.
    /// </summary>
    public ValueList<string> Environment { get; }

    /// <summary>The working directory; empty inherits the service's.</summary>
    public string WorkingDirectory { get; }

    /// <summary>What standard input carries: <c>json</c>, <c>none</c>, <c>csv</c> or <c>template</c>.</summary>
    public string Stdin { get; }

    /// <summary>The bytes standard input would carry.</summary>
    public string StdinData { get; }

    /// <summary>The file the invocation's <c>inputFile</c> writes; null when it has none.</summary>
    public ToolDryRunFile? InputFile { get; }

    /// <summary>Whether the entry uses <c>{outputDir}</c>.</summary>
    public bool OutputDirectory { get; }

    /// <summary>The values the call sends the tool.</summary>
    public ValueList<ToolDryRunInput> Inputs { get; }

    /// <summary>The bindings the call expects back.</summary>
    public ValueList<ToolDryRunOutput> Outputs { get; }

    /// <summary>Where the reply is read: <c>stdout</c>, or <c>file:&lt;template&gt;</c>.</summary>
    public string ReplySource { get; }

    /// <summary>The preview as the command prints it, one line each, the reply mapping included.</summary>
    public ValueList<string> Lines { get; }

    /// <summary>The preview as the command prints it.</summary>
    /// <returns>The preview's text.</returns>
    public override string ToString() => string.Join('\n', Lines);
}

