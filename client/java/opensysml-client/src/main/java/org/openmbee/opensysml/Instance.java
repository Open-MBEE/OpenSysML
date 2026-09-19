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
 */
public record Instance(long id, String typeSymbolId, Map<String, Instance.FeatureValue> featureValues) {

  /**
   * Creates an instance, copying its feature values.
   *
   * @param id id of the instance
   * @param typeSymbolId FQN of its type
   * @param featureValues feature values by feature name
   */
  public Instance {
    Objects.requireNonNull(typeSymbolId, "typeSymbolId");
    featureValues = Map.copyOf(featureValues);
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
