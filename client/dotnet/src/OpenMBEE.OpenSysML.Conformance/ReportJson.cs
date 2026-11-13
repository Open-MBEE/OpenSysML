using System;
using System.Collections.Generic;
using System.IO;
using System.Text.Encodings.Web;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Writes a report as the JSON the reference runner writes with <c>-report</c>: the same members,
/// spelled and ordered the same, so reports of the two runners compare member by member.
/// Optional members are left out when empty, as the reference leaves them out.
/// </summary>
internal static class ReportJson
{
    private static readonly JsonWriterOptions Options = new()
    {
        Indented = true,
        IndentSize = 2,
        NewLine = "\n",
        Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping,
    };

    /// <summary>Writes a report, followed by a newline.</summary>
    /// <param name="report">The report.</param>
    /// <param name="stream">Where to write it, as UTF-8.</param>
    internal static void Write(ConformanceReport report, Stream stream)
    {
        ArgumentNullException.ThrowIfNull(report);
        ArgumentNullException.ThrowIfNull(stream);
        using (var json = new Utf8JsonWriter(stream, Options))
        {
            json.WriteStartObject();
            json.WriteString("service", report.Service);
            WriteTotals(json, report.Totals);
            json.WriteStartArray("configurations");
            foreach (var configuration in report.Configurations)
            {
                WriteConfiguration(json, configuration);
            }

            json.WriteEndArray();
            json.WriteEndObject();
        }

        stream.WriteByte((byte)'\n');
    }

    private static void WriteConfiguration(Utf8JsonWriter json, ConfigurationSummary configuration)
    {
        json.WriteStartObject();
        json.WriteString("name", configuration.Name);
        if (configuration.UnavailableCapabilities.Count > 0)
        {
            WriteStrings(json, "unavailable_capabilities", configuration.UnavailableCapabilities);
        }

        WriteTotals(json, configuration.Totals);
        json.WriteStartArray("protocols");
        foreach (var protocol in configuration.Protocols)
        {
            WriteProtocol(json, protocol);
        }

        json.WriteEndArray();
        json.WriteEndObject();
    }

    private static void WriteProtocol(Utf8JsonWriter json, ProtocolSummary protocol)
    {
        json.WriteStartObject();
        json.WriteString("protocol", protocol.Protocol);
        json.WriteString("service", protocol.Service);
        WriteStrings(json, "capabilities", protocol.Capabilities);
        WriteTotals(json, protocol.Totals);
        json.WriteStartArray("results");
        foreach (var result in protocol.Results)
        {
            WriteResult(json, result);
        }

        json.WriteEndArray();
        json.WriteEndObject();
    }

    private static void WriteResult(Utf8JsonWriter json, ScenarioResult result)
    {
        json.WriteStartObject();
        json.WriteString("id", result.Id);
        json.WriteString("outcome", OutcomeName(result.Outcome));
        json.WriteString("rpc", result.Rpc);
        if (result.Reason.Length > 0)
        {
            json.WriteString("reason", result.Reason);
        }

        if (result.Failures.Count > 0)
        {
            WriteStrings(json, "failures", result.Failures);
        }

        json.WriteString("status", result.Status);
        if (result.WithoutCapability)
        {
            json.WriteBoolean("without_capability", true);
        }

        if (result.MissingCapabilities.Count > 0)
        {
            WriteStrings(json, "missing_capabilities", result.MissingCapabilities);
        }

        json.WriteNumber("duration_ms", result.DurationMs);
        json.WriteEndObject();
    }

    /// <summary>An outcome as a report spells it: <c>pass</c>, <c>fail</c>, <c>skip</c> or <c>error</c>.</summary>
    /// <param name="outcome">The outcome.</param>
    /// <returns>Its name.</returns>
    internal static string OutcomeName(ScenarioOutcome outcome) => outcome switch
    {
        ScenarioOutcome.Pass => "pass",
        ScenarioOutcome.Fail => "fail",
        ScenarioOutcome.Skip => "skip",
        ScenarioOutcome.Error => "error",
        _ => throw new ArgumentOutOfRangeException(nameof(outcome), outcome, "no such outcome"),
    };

    private static void WriteTotals(Utf8JsonWriter json, Totals totals)
    {
        json.WriteNumber("total", totals.Total);
        json.WriteNumber("passed", totals.Passed);
        json.WriteNumber("failed", totals.Failed);
        json.WriteNumber("skipped", totals.Skipped);
        json.WriteNumber("errored", totals.Errored);
        json.WriteNumber("without_capability", totals.WithoutCapability);
    }

    private static void WriteStrings(Utf8JsonWriter json, string name, IEnumerable<string> values)
    {
        json.WriteStartArray(name);
        foreach (var value in values)
        {
            json.WriteStringValue(value);
        }

        json.WriteEndArray();
    }
}
