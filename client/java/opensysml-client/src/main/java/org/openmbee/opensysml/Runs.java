package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * The table {@link Model#runRuns} returns: one row per run, every observed feature summarised
 * over the runs, and for a Monte Carlo case its conclusion over the sample.
 *
 * @param rows one row per run, in run order
 * @param observables what every row reports, in column order
 * @param distributions each observable over the completed runs
 * @param outputs a Monte Carlo case's conclusion over the sample, empty for an action
 * @param verdicts the conclusion's checks
 * @param unconcluded why the case could not be concluded; empty when it was
 * @param seed the seed the random draws started from, absent when they were fixed
 * @param instances the objects the conclusion names
 * @param diagnostics notes raised
 * @param standing the engine's name, strength and bounds
 */
public record Runs(
    List<SweepRow> rows,
    List<String> observables,
    List<Distribution> distributions,
    Map<String, Value> outputs,
    List<Verdict> verdicts,
    String unconcluded,
    Optional<Long> seed,
    List<Instance> instances,
    List<Diagnostic> diagnostics,
    Standing standing) {
  /** Validates and defensively copies the collections. */
  public Runs {
    rows = List.copyOf(rows);
    observables = List.copyOf(observables);
    distributions = List.copyOf(distributions);
    outputs = Collections.unmodifiableMap(new LinkedHashMap<>(outputs));
    verdicts = List.copyOf(verdicts);
    Objects.requireNonNull(unconcluded, "unconcluded");
    Objects.requireNonNull(seed, "seed");
    instances = List.copyOf(instances);
    diagnostics = List.copyOf(diagnostics);
    Objects.requireNonNull(standing, "standing");
  }

  /**
   * How many runs were made.
   *
   * @return the row count
   */
  public int size() {
    return rows.size();
  }

  /**
   * Whether every run completed.
   *
   * @return true if no row carries an error
   */
  public boolean completed() {
    return rows.stream().noneMatch(SweepRow::failed);
  }

  /**
   * One output of a Monte Carlo case's conclusion.
   *
   * @param name the output's name
   * @return its value, or absent
   */
  public Optional<Value> output(String name) {
    return Optional.ofNullable(outputs.get(name));
  }

  /**
   * One observable over the completed runs.
   *
   * @param name the observable
   * @return its distribution, or absent
   */
  public Optional<Distribution> distribution(String name) {
    return distributions.stream().filter(d -> d.name().equals(name)).findFirst();
  }

  /**
   * Finds an instance the conclusion names.
   *
   * @param instanceId the instance's identifier
   * @return the instance, or absent
   */
  public Optional<Instance> instance(long instanceId) {
    return Instances.find(instances, instanceId);
  }

  /**
   * One observable over the completed runs.
   *
   * @param name the observable
   * @param unit the unit every value carried; empty for none
   * @param count how many runs reported it
   * @param numeric whether the values are numbers, summarised by the statistics
   * @param min the least value of a numeric observable
   * @param max the greatest value of a numeric observable
   * @param mean the mean of a numeric observable
   * @param deviation the standard deviation of a numeric observable
   * @param percentiles the percentiles of a numeric observable
   * @param histogram the histogram of a numeric observable
   * @param values each distinct value of a non-numeric observable, counted
   */
  public record Distribution(
      String name,
      String unit,
      long count,
      boolean numeric,
      Optional<Value> min,
      Optional<Value> max,
      Optional<Value> mean,
      double deviation,
      List<Percentile> percentiles,
      List<HistogramBin> histogram,
      List<CountedValue> values) {
    /** Validates and defensively copies the collections. */
    public Distribution {
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(unit, "unit");
      Objects.requireNonNull(min, "min");
      Objects.requireNonNull(max, "max");
      Objects.requireNonNull(mean, "mean");
      percentiles = List.copyOf(percentiles);
      histogram = List.copyOf(histogram);
      values = List.copyOf(values);
    }
  }

  /**
   * One percentile of a numeric observable.
   *
   * @param percentile the percentile, such as 50.0
   * @param value the value
   */
  public record Percentile(double percentile, Value value) {
    /** Validates the value. */
    public Percentile {
      Objects.requireNonNull(value, "value");
    }
  }

  /**
   * One bin of a numeric observable's histogram.
   *
   * @param low the lower bound, inclusive
   * @param high the upper bound, exclusive
   * @param count how many runs fell in the bin
   */
  public record HistogramBin(Value low, Value high, long count) {
    /** Validates the bounds. */
    public HistogramBin {
      Objects.requireNonNull(low, "low");
      Objects.requireNonNull(high, "high");
    }
  }

  /**
   * One distinct value of a non-numeric observable.
   *
   * @param value the value
   * @param count how many runs reported it
   */
  public record CountedValue(Value value, long count) {
    /** Validates the value. */
    public CountedValue {
      Objects.requireNonNull(value, "value");
    }
  }
}
