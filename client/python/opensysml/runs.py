"""Results of repeated runs, the reflective self-check and run recording:
what ``RunRuns``, ``SelfCheck`` and ``RecordRun`` answer, as Python values."""

from opensysml.values import value_to_python
from opensysml.engines import Standing


def _value(pb_value, resolve=None):
    if resolve is None:
        resolve = lambda instance_id: instance_id  # noqa: E731
    return value_to_python(pb_value, resolve)


class RunPercentile:
    """The value a percentile of the runs fell at or below.

    Attributes:
        percentile (float): The percentile, e.g. 50.0
        value: The value at it
    """

    def __init__(self, percentile, value):
        self.percentile = percentile
        self.value = value

    @classmethod
    def of(cls, pb):
        return cls(pb.percentile, _value(pb.value))


class RunHistogramBin:
    """How many runs observed a value in ``[low, high)``."""

    def __init__(self, low, high, count):
        self.low = low
        self.high = high
        self.count = count

    @classmethod
    def of(cls, pb):
        return cls(_value(pb.low), _value(pb.high), pb.count)


class RunCountedValue:
    """How many runs observed one distinct value."""

    def __init__(self, value, count):
        self.value = value
        self.count = count

    @classmethod
    def of(cls, pb):
        return cls(_value(pb.value), pb.count)


class RunDistribution:
    """One observable over the completed runs.

    Attributes:
        name (str): The observable
        unit (str): The unit every run's value carried; empty for none
        count (int): Runs that observed it
        numeric (bool): Whether the values are numbers, summarised below
        min, max, mean: The extremes and mean of a numeric observable
        deviation (float): Its standard deviation
        percentiles (list[RunPercentile]): Its percentiles
        histogram (list[RunHistogramBin]): Its histogram
        values (list[RunCountedValue]): Each distinct value of a non-numeric
            observable with how many runs observed it
    """

    def __init__(self, name, unit, count, numeric, minimum=None, maximum=None, mean=None,
                 deviation=0.0, percentiles=(), histogram=(), values=()):
        self.name = name
        self.unit = unit
        self.count = count
        self.numeric = numeric
        self.min = minimum
        self.max = maximum
        self.mean = mean
        self.deviation = deviation
        self.percentiles = list(percentiles)
        self.histogram = list(histogram)
        self.values = list(values)

    @classmethod
    def of(cls, pb):
        numbers = {}
        if pb.numeric and pb.HasField("min"):
            numbers = {"minimum": _value(pb.min), "maximum": _value(pb.max), "mean": _value(pb.mean)}
        return cls(
            pb.name, pb.unit, pb.count, pb.numeric, deviation=pb.deviation,
            percentiles=[RunPercentile.of(p) for p in pb.percentiles],
            histogram=[RunHistogramBin.of(b) for b in pb.histogram],
            values=[RunCountedValue.of(v) for v in pb.values],
            **numbers,
        )


class RunsResult:
    """What ``run_runs`` answers: the table of runs, each observable's
    distribution and, for a Monte Carlo case, its conclusion over the sample.

    Attributes:
        rows (list[SweepRow]): One per run, in run order; its inputs are the
            run number, its outputs what it observed
        observables (list[str]): What every row reports, in column order
        distributions (list[RunDistribution]): Each observable over the
            completed runs
        lines (list[str]): The report as ``sysml -runs`` prints it
        outputs (dict): A Monte Carlo case's conclusion over the sample
        verdicts (list[Verdict]): The conclusion's checks
        unconcluded (str): Why the case has no conclusion; empty when it has one
        instances (list[Instance]): The objects the conclusion names
        standing (Standing): The engine's name, strength and bounds
        seed (int or None): The seed the runs drew from; None when the draws
            were fixed
        diagnostics (list[Diagnostic]): Notes the runs raised
    """

    def __init__(self, rows, observables, distributions, lines, outputs, verdicts, unconcluded,
                 instances, standing, seed, diagnostics):
        self.rows = list(rows)
        self.observables = list(observables)
        self.distributions = list(distributions)
        self.lines = list(lines)
        self.outputs = dict(outputs)
        self.verdicts = list(verdicts)
        self.unconcluded = unconcluded
        self.instances = list(instances)
        self.standing = standing
        self.seed = seed
        self.diagnostics = list(diagnostics)

    def __len__(self):
        return len(self.rows)

    def __iter__(self):
        return iter(self.rows)

    def __str__(self):
        return "\n".join(self.lines)


class SelfCheckFinding:
    """One thing the reflective check reports.

    Attributes:
        status (str): ``fails``, ``unevaluated``, ``error`` or ``warning``
        subject (str): ``<constraint> for <element>``, or the constraint or
            package alone
        location (str): ``document:line:col`` of the element; empty for none
        constraint (str): The constraint applied
        element (str): The element it was applied to
        message (str): The reason, for a warning or an error
        line (str): The finding as ``sysml -self-check`` prints it
    """

    def __init__(self, status, subject, location, constraint, element, message, line):
        self.status = status
        self.subject = subject
        self.location = location
        self.constraint = constraint
        self.element = element
        self.message = message
        self.line = line

    @classmethod
    def of(cls, pb):
        return cls(pb.status, pb.subject, pb.location, pb.constraint, pb.element, pb.message, pb.line)

    def __str__(self):
        return self.line


class SelfCheckResult:
    """What ``self_check`` answers: every finding and the counts over them.

    Attributes:
        status (str): ``holds``, ``fails`` or ``unresolved``
        findings (list[SelfCheckFinding]): Each violation, evaluation the
            runtime could not make, error and skipped rule constraint
        elements, checks, violations, unevaluated, evaluation_errors (int):
            The counts
        applications (dict[str, int]): How many elements each constraint was
            applied to
        summary (str): The one-line total ``sysml -self-check`` prints last
        diagnostics (list[Diagnostic]): Notes the check raised
    """

    def __init__(self, status, findings, elements, checks, violations, unevaluated,
                 evaluation_errors, applications, summary, diagnostics):
        self.status = status
        self.findings = list(findings)
        self.elements = elements
        self.checks = checks
        self.violations = violations
        self.unevaluated = unevaluated
        self.evaluation_errors = evaluation_errors
        self.applications = dict(applications)
        self.summary = summary
        self.diagnostics = list(diagnostics)

    @property
    def holds(self):
        """Whether the check found no violation and no evaluation error."""
        return self.status == "holds"

    def __str__(self):
        return "\n".join([f.line for f in self.findings] + [self.summary])


class RecordedDocument:
    """One document the records were written into: its name and notation."""

    def __init__(self, name, content):
        self.name = name
        self.content = content


class Recording:
    """What ``record_run`` answers: the runs' own report, what the model
    gained and the notation the records were written into.

    Attributes:
        run (AnalysisResult or None): The run's result, for a case run once
        sweep (SweepTable or None): The rows, for a case run over ranges
        sample (RunsResult or None): The runs, for a Monte Carlo sample
        package (str): The package the records went into
        definition (str): The per-case record definition
        records (list[str]): The records made, in order
        source (str): The declarations generated
        content (str): The recorded notation of a single-document model the
            records merged into; empty otherwise
        documents (list[RecordedDocument]): The document the records went
            into, or the records as a document of their own
        skipped (int): Rows or runs that failed and were not recorded
        reasons (list[str]): Why runs were not recorded
        model_hash (str): The recorded model, cached for the next request
        diagnostics (list[Diagnostic]): Notes the recording raised
    """

    def __init__(self, run, sweep, sample, package, definition, records, source, content,
                 documents, skipped, reasons, model_hash, diagnostics):
        self.run = run
        self.sweep = sweep
        self.sample = sample
        self.package = package
        self.definition = definition
        self.records = list(records)
        self.source = source
        self.content = content
        self.documents = list(documents)
        self.skipped = skipped
        self.reasons = list(reasons)
        self.model_hash = model_hash
        self.diagnostics = list(diagnostics)


def self_check_result(response):
    """Build a :class:`SelfCheckResult` from a ``SelfCheckResponse``."""
    from opensysml.diagnostic import Diagnostic

    return SelfCheckResult(
        response.status,
        [SelfCheckFinding.of(f) for f in response.findings],
        response.elements, response.checks, response.violations, response.unevaluated,
        response.evaluation_errors,
        {a.constraint: a.count for a in response.applications},
        response.summary,
        [Diagnostic(d) for d in response.diagnostics],
    )


__all__ = [
    "RecordedDocument", "Recording", "RunCountedValue", "RunDistribution", "RunHistogramBin",
    "RunPercentile", "RunsResult", "SelfCheckFinding", "SelfCheckResult", "Standing",
]
