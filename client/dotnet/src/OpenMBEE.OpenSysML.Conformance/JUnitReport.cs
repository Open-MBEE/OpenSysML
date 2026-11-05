using System;
using System.IO;
using System.Linq;
using System.Text;
using System.Xml;
using System.Xml.Linq;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Writes a report as the JUnit XML the reference runner writes with <c>-junit</c>, the format CI
/// systems render as a test report: one suite per configuration and protocol, named
/// <c>configuration/protocol</c>, and one case per scenario. A failure carries the mismatches, an
/// error and a skip their reason; times are in seconds.
/// </summary>
internal static class JUnitReport
{
    private const string Declaration = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n";

    private static readonly XmlWriterSettings Settings = new()
    {
        OmitXmlDeclaration = true,
        Indent = true,
        IndentChars = "  ",
        NewLineChars = "\n",
        NewLineHandling = NewLineHandling.Entitize,
    };

    /// <summary>Writes a report, followed by a newline.</summary>
    /// <param name="report">The report.</param>
    /// <param name="stream">Where to write it, as UTF-8.</param>
    internal static void Write(ConformanceReport report, Stream stream)
    {
        ArgumentNullException.ThrowIfNull(report);
        ArgumentNullException.ThrowIfNull(stream);

        var text = new StringBuilder(Declaration);
        using (var writer = XmlWriter.Create(text, Settings))
        {
            Document(report).WriteTo(writer);
        }

        var bytes = new UTF8Encoding(encoderShouldEmitUTF8Identifier: false).GetBytes(text.Append('\n').ToString());
        stream.Write(bytes);
    }

    private static XElement Document(ConformanceReport report)
    {
        var suites = report.Configurations
            .SelectMany(configuration => configuration.Protocols.Select(protocol => Suite($"{configuration.Name}/{protocol.Protocol}", protocol)))
            .ToList();
        var time = 0.0;
        foreach (var suite in suites)
        {
            time += suite.Time;
        }

        return new XElement(
            "testsuites",
            new XAttribute("name", "conformance"),
            Counts(suites.Sum(suite => suite.Tests), suites.Sum(suite => suite.Failures), suites.Sum(suite => suite.Errors), suites.Sum(suite => suite.Skipped), time),
            suites.Select(suite => suite.Element));
    }

    private static (XElement Element, int Tests, int Failures, int Errors, int Skipped, double Time) Suite(string name, ProtocolSummary protocol)
    {
        var time = 0.0;
        foreach (var result in protocol.Results)
        {
            time += Seconds(result);
        }

        var failures = protocol.Results.Count(result => result.Outcome == ScenarioOutcome.Fail);
        var errors = protocol.Results.Count(result => result.Outcome == ScenarioOutcome.Error);
        var skipped = protocol.Results.Count(result => result.Outcome == ScenarioOutcome.Skip);
        var element = new XElement(
            "testsuite",
            new XAttribute("name", XmlText(name)),
            Counts(protocol.Results.Count, failures, errors, skipped, time),
            protocol.Results.Select(result => Case(name, result)));
        return (element, protocol.Results.Count, failures, errors, skipped, time);
    }

    private static XElement Case(string suite, ScenarioResult result)
    {
        var element = new XElement(
            "testcase",
            new XAttribute("name", XmlText(result.Id)),
            new XAttribute("classname", XmlText(suite)),
            new XAttribute("time", Spelling.Real(Seconds(result))));
        var details = string.Join("\n", result.Failures);
        switch (result.Outcome)
        {
            case ScenarioOutcome.Fail:
                element.Add(Message("failure", result.Reason, details));
                break;
            case ScenarioOutcome.Error:
                element.Add(Message("error", result.Reason, details));
                break;
            case ScenarioOutcome.Skip:
                element.Add(Message("skipped", result.Reason, ""));
                break;
        }

        return element;
    }

    private static XElement Message(string kind, string text, string body)
    {
        var element = new XElement(kind);
        if (text.Length > 0)
        {
            element.Add(new XAttribute("message", XmlText(text)));
        }

        if (body.Length > 0)
        {
            element.Add(new XText(XmlText(body)));
        }

        return element;
    }

    private static string XmlText(string text)
    {
        var safe = new StringBuilder(text.Length);
        for (var index = 0; index < text.Length; index++)
        {
            var character = text[index];
            if (XmlConvert.IsXmlChar(character))
            {
                safe.Append(character);
            }
            else if (char.IsHighSurrogate(character) && index + 1 < text.Length && char.IsLowSurrogate(text[index + 1]))
            {
                safe.Append(character).Append(text[++index]);
            }
            else
            {
                safe.Append('\uFFFD');
            }
        }

        return safe.ToString();
    }

    private static XAttribute[] Counts(int tests, int failures, int errors, int skipped, double time) =>
    [
        new("tests", tests),
        new("failures", failures),
        new("errors", errors),
        new("skipped", skipped),
        new("time", Spelling.Real(time)),
    ];

    private static double Seconds(ScenarioResult result) => result.DurationMs / 1000;
}
