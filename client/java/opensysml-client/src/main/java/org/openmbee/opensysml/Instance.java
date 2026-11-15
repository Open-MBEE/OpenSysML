package org.openmbee.opensysml;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * A runtime object the service built for a part or usage.
 *
 * @param id id the service assigned it, which {@link Value.InstanceReference} refers to
 * @param typeSymbolId FQN of the definition or usage it is an object of
 * @param featureValues what it holds for each feature of its type, by feature name
 * @param reachedBy how the traversal that built the object graph reached this object; absent for
 *     the root, and from a service without {@code instance_paths}
 */
public record Instance(
    long id,
    String typeSymbolId,
    Map<String, Instance.FeatureValue> featureValues,
    Optional<Instance.Reach> reachedBy) {

  /**
   * Creates an instance, copying its feature values.
   *
   * @param id id of the instance
   * @param typeSymbolId FQN of its type
   * @param featureValues feature values by feature name
   * @param reachedBy how the traversal reached it, when it is not the root
   */
  public Instance {
    Objects.requireNonNull(typeSymbolId, "typeSymbolId");
    featureValues = Map.copyOf(featureValues);
    Objects.requireNonNull(reachedBy, "reachedBy");
  }

  /**
   * Creates a root instance, or one of a service that does not say how it was reached.
   *
   * @param id id of the instance
   * @param typeSymbolId FQN of its type
   * @param featureValues feature values by feature name
   */
  public Instance(long id, String typeSymbolId, Map<String, Instance.FeatureValue> featureValues) {
    this(id, typeSymbolId, featureValues, Optional.empty());
  }

  /**
   * The object holding this one along the path the traversal reached it by. One object is often a
   * value of several features (a port and the connector end referencing it are one object); this
   * is the holder the traversal came through, and {@link Model#instancePaths(Instance)} lists
   * every path to it.
   *
   * @return a reference {@link Instantiation#resolve} turns into the object, absent for the root
   */
  public Optional<Value.InstanceReference> owner() {
    return reachedBy.map(reach -> new Value.InstanceReference(reach.instanceId()));
  }

  /**
   * The path from the root the traversal reached this object by: the feature names, indexed within
   * a multi-valued feature, joined by dots ({@code "powertrain.engine.fuelIn"},
   * {@code "wheels[2].hub"}).
   *
   * @return the path, empty for the root
   */
  public String path() {
    return reachedBy.map(Reach::path).orElse("");
  }

  /**
   * How a traversal of an object graph reached one object. The traversal is depth-first from the
   * root, over the features of an object in name order and the values of a feature in their order,
   * reaching an object once.
   *
   * @param instanceId the object holding this one
   * @param featureName the holder's feature, a key of its {@link Instance#featureValues()}
   * @param featureSymbolId the FQN of the feature as declared
   * @param index the position among the values of a multi-valued feature, counted from 1 as an
   *     object path indexes them; 0 for a single-valued one
   * @param path the path from the root, the feature names joined by dots and indexed within a
   *     multi-valued feature
   */
  public record Reach(
      long instanceId, String featureName, String featureSymbolId, int index, String path) {
    /**
     * Creates a reach.
     *
     * @param instanceId the holder
     * @param featureName the holder's feature, never {@code null}
     * @param featureSymbolId the feature's FQN
     * @param index the position, 0 for single-valued
     * @param path the path from the root
     */
    public Reach {
      Objects.requireNonNull(featureName, "featureName");
      Objects.requireNonNull(featureSymbolId, "featureSymbolId");
      Objects.requireNonNull(path, "path");
    }
  }

  /**
   * The value of one feature, single- or multi-valued.
   *
   * @param featureName name of the feature
   * @param value the value of a single-valued feature, absent for a multi-valued one or a failure
   * @param values the values of a multi-valued feature, empty for a single-valued one
   * @param materialized whether the object holds the feature at all
   * @param error why evaluation failed, absent when it did not
   */
  public record FeatureValue(
      String featureName,
      Optional<Value> value,
      List<Value> values,
      boolean materialized,
      Optional<String> error) {
    /**
     * Creates a feature value, copying the value list.
     *
     * @param featureName name of the feature, never {@code null}
     * @param value single value, when there is one
     * @param values multiple values
     * @param materialized whether the feature is held
     * @param error failure reason, when evaluation failed
     */
    public FeatureValue {
      Objects.requireNonNull(featureName, "featureName");
      Objects.requireNonNull(value, "value");
      values = List.copyOf(values);
      Objects.requireNonNull(error, "error");
    }
  }
}
