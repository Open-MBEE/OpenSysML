"""Typed answers of ``import_data`` and ``compare_results``."""

from opensysml.diagnostic import Diagnostic
from opensysml.engines import Standing


class ImportedValue:
    """One assignment an import made.

    Attributes:
        element (str): Qualified name of the element whose feature is set
        feature (str): The feature set
        value (str): The literal it is set to; ``""`` for a redefinition made
            only to reach a nested inherited feature
        redefines (str): Qualified name of the inherited feature redefined to
            set it; ``""`` when the element declares the feature itself
        where (str): The row or cell of the data it came from
        description (str): The line ``sysml -import`` reports it as
    """

    def __init__(self, element, feature, value, redefines, where, description):
        self.element = element
        self.feature = feature
        self.value = value
        self.redefines = redefines
        self.where = where
        self.description = description

    def __repr__(self):
        return f"ImportedValue({self.description!r})"


class ImportedFile:
    """What one data file set.

    Attributes:
        name (str): The file's name
        values (int): How many values it set
        elements (int): How many elements they were set on
        assignments (list[ImportedValue]): Each assignment, in data order
    """

    def __init__(self, name, values, elements, assignments):
        self.name = name
        self.values = values
        self.elements = elements
        self.assignments = list(assignments)

    def __repr__(self):
        return f"ImportedFile({self.name!r}, values={self.values}, elements={self.elements})"


class ImportResult:
    """What ``import_data`` answers: what each file set and the model it made.

    Attributes:
        files (list[ImportedFile]): One per data file, in order
        content (str): The edited content of a one-document model; ``""`` on
            a dry run
        documents (list[EditedDocument]): Every edited document; empty on a
            dry run
        model_hash (str): The imported model's hash, parsed and cached by the
            service; ``""`` on a dry run
        diagnostics (list[Diagnostic]): Notes the import raised
    """

    def __init__(self, files, content, documents, model_hash, diagnostics):
        self.files = list(files)
        self.content = content
        self.documents = list(documents)
        self.model_hash = model_hash
        self.diagnostics = list(diagnostics)

    @property
    def values(self):
        """How many values every file set together."""
        return sum(f.values for f in self.files)

    def __repr__(self):
        return f"ImportResult(files={self.files!r}, model_hash={self.model_hash!r})"


class ComparedDistribution:
    """A distribution of numbers by its statistics; a pooled one knows its
    ``count`` and ``mean`` alone.

    Attributes:
        count (int): How many numbers the distribution pools
        min (float): The smallest number
        mean (float): The arithmetic mean
        p50 (float): The median
        p90 (float): The 90th percentile
        max (float): The largest number
        deviation (float): The standard deviation
    """

    def __init__(self, count, min, mean, p50, p90, max, deviation):
        self.count = count
        self.min = min
        self.mean = mean
        self.p50 = p50
        self.p90 = p90
        self.max = max
        self.deviation = deviation

    def __repr__(self):
        return f"ComparedDistribution(count={self.count}, mean={self.mean})"


class StatisticDifference:
    """The relative difference of each statistic as a signed percentage, ``""``
    where it is not compared.

    Attributes:
        min (str): The difference of the minimum
        mean (str): The difference of the mean
        p50 (str): The difference of the median
        p90 (str): The difference of the 90th percentile
        max (str): The difference of the maximum
    """

    def __init__(self, min, mean, p50, p90, max):
        self.min = min
        self.mean = mean
        self.p50 = p50
        self.p90 = p90
        self.max = max

    def __repr__(self):
        return f"StatisticDifference(mean={self.mean!r})"


class CaseStatistic:
    """One statistic a ``Simulation::MonteCarlo`` case declares: which, where
    the case declares it, the tool's value, the runs' and their difference.

    Attributes:
        statistic (str): Which statistic the case declares
        of (str): The feature the statistic is of
        tool (str): The tool's value
        run (str): The runs' value
        difference (str): The relative difference, ``""`` where not compared
    """

    def __init__(self, statistic, of, tool, run, difference):
        self.statistic = statistic
        self.of = of
        self.tool = tool
        self.run = run
        self.difference = difference


class CaseStatistics:
    """The statistics a ``Simulation::MonteCarlo`` case declares of an
    observable, the tool's beside the runs'.

    Attributes:
        analysis (str): The case
        rows (list[CaseStatistic]): One per statistic
        notes (list[str]): What was not compared, and why
    """

    def __init__(self, analysis, rows, notes):
        self.analysis = analysis
        self.rows = list(rows)
        self.notes = list(notes)


class ObservableComparison:
    """One observable: the tool's distribution beside the runs', or the notes
    on why they are not compared.

    Attributes:
        stored (str): The observable the tool stored
        feature (str): The feature of the run that answers it
        unit (str): The unit the runs' numbers came in, ``""`` for none
        tool (ComparedDistribution | None): The tool's distribution
        pooled (bool): Whether it was pooled from summaries
        unstored (bool): Whether the tool stored no result at all
        run (ComparedDistribution | None): The completed runs' distribution
        produced (int): How many runs produced a number for the feature
        compared (bool): Whether the runs were made and are reported
        difference (StatisticDifference | None): Per statistic
        declared (CaseStatistics | None): The case's statistics compared
        notes (list[str]): What the comparison remarks on
    """

    def __init__(self, stored, feature, unit, tool, pooled, unstored, run, produced,
                 compared, difference, declared, notes):
        self.stored = stored
        self.feature = feature
        self.unit = unit
        self.tool = tool
        self.pooled = pooled
        self.unstored = unstored
        self.run = run
        self.produced = produced
        self.compared = compared
        self.difference = difference
        self.declared = declared
        self.notes = list(notes)

    def __repr__(self):
        return f"ObservableComparison({self.stored!r} by {self.feature!r}, compared={self.compared})"


class ConfigurationComparison:
    """One configuration's runs beside the tool's results.

    Attributes:
        id (str): The configuration's identifier in the sidecar
        name (str): The configuration's name
        behavior (str): The behavior it runs
        target (str): The usage it runs against
        result_location (str): Where the sidecar keeps its results
        status (str): ``holds``, ``fails`` (a run failed) or ``unresolved``
        unresolved (str): Why the configuration could not be run
        stored_runs (int): How many runs the tool stored
        stored_snapshots (int): How many snapshots the tool stored
        runs (int): The runs OpenSysML made
        completed (int): How many of them completed
        draws (str): The draw policy the runs took
        seed (int | None): The seed the runs derived theirs from
        clock_step (float): The clock step, 0 for a continuous clock
        rows (list[SweepRow]): One per run
        observables (list[ObservableComparison]): One per observed feature
        errors (list[str]): The errors failed runs ended in
        notes (list[str]): What the comparison remarks on
        lines (list[str]): The comparison as ``sysml -compare-results`` prints it
    """

    def __init__(self, id, name, behavior, target, result_location, status, unresolved,
                 stored_runs, stored_snapshots, runs, completed, draws, seed, clock_step,
                 rows, observables, errors, notes, lines):
        self.id = id
        self.name = name
        self.behavior = behavior
        self.target = target
        self.result_location = result_location
        self.status = status
        self.unresolved = unresolved
        self.stored_runs = stored_runs
        self.stored_snapshots = stored_snapshots
        self.runs = runs
        self.completed = completed
        self.draws = draws
        self.seed = seed
        self.clock_step = clock_step
        self.rows = list(rows)
        self.observables = list(observables)
        self.errors = list(errors)
        self.notes = list(notes)
        self.lines = list(lines)

    @property
    def holds(self):
        """Whether every run completed and the observables were compared."""
        return self.status == "holds"

    def __str__(self):
        return "\n".join(self.lines)

    def __repr__(self):
        return f"ConfigurationComparison({self.name!r}, status={self.status!r})"


class Comparison:
    """What ``compare_results`` answers: one configuration after another, in
    the order the sidecar indexes them.

    Attributes:
        source (str): The tool the sidecar records as the results' source
        configurations (list[ConfigurationComparison]): One per configuration
        standing (Standing): The engine and strength of the last runs made
        diagnostics (list[Diagnostic]): The diagnostics the comparison raised
    """

    def __init__(self, source, configurations, standing, diagnostics):
        self.source = source
        self.configurations = list(configurations)
        self.standing = standing
        self.diagnostics = list(diagnostics)

    @property
    def holds(self):
        """Whether every configuration ran and compared."""
        return all(c.holds for c in self.configurations)

    def __str__(self):
        return "\n".join(str(c) for c in self.configurations)

    def __repr__(self):
        return f"Comparison({self.source!r}, {len(self.configurations)} configuration(s))"


def import_result(response, documents):
    """The ImportResult of an ImportDataResponse, with its edited documents decoded."""
    files = [
        ImportedFile(f.name, f.values, f.elements, [
            ImportedValue(a.element, a.feature, a.value, a.redefines, a.where, a.description)
            for a in f.assignments
        ])
        for f in response.files
    ]
    return ImportResult(files, response.content, documents, response.model_hash,
                        [Diagnostic(d) for d in response.diagnostics])


def _distribution(d):
    if d is None:
        return None
    return ComparedDistribution(d.count, d.min, d.mean, d.p50, d.p90, d.max, d.deviation)


def comparison_result(response, rows_of):
    """The Comparison of a CompareResultsResponse; ``rows_of`` decodes a
    configuration's rows and instances into SweepRows."""
    configurations = []
    for c in response.configurations:
        observables = []
        for o in c.observables:
            difference = None
            if o.HasField("difference"):
                d = o.difference
                difference = StatisticDifference(d.min, d.mean, d.p50, d.p90, d.max)
            declared = None
            if o.HasField("declared"):
                declared = CaseStatistics(o.declared.analysis, [
                    CaseStatistic(r.statistic, r.of, r.tool, r.run, r.difference) for r in o.declared.rows
                ], o.declared.notes)
            observables.append(ObservableComparison(
                o.stored, o.feature, o.unit,
                _distribution(o.tool) if o.HasField("tool") else None, o.pooled, o.unstored,
                _distribution(o.run) if o.HasField("run") else None, o.produced, o.compared,
                difference, declared, o.notes,
            ))
        configurations.append(ConfigurationComparison(
            c.id, c.name, c.behavior, c.target, c.result_location, c.status, c.unresolved,
            c.stored_runs, c.stored_snapshots, c.runs, c.completed, c.draws,
            c.seed if c.HasField("seed") else None, c.clock_step,
            rows_of(c.rows, c.instances), observables, c.errors, c.notes, c.lines,
        ))
    return Comparison(response.source, configurations, Standing.of(response), [Diagnostic(d) for d in response.diagnostics])
