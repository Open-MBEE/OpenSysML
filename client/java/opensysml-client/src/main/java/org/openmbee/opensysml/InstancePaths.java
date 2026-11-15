package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * Every root-relative path that reaches one object, aliases through connector ends and reference
 * usages included, as {@link Model#instancePaths(Instance)} answers it.
 *
 * @param paths the paths in traversal order: the roots the service holds in id order and, from
 *     each, depth-first over the features of an object in name order and the values of a feature
 *     in their order
 * @param truncated whether the service bounded its search short of every path: more reach the
 *     object than it lists, or one deeper than it walks
 */
public record InstancePaths(List<InstancePaths.Path> paths, boolean truncated) {
  /**
   * Creates the answer, copying the paths.
   *
   * @param paths the paths
   * @param truncated whether the search was bounded
   */
  public InstancePaths {
    paths = List.copyOf(paths);
  }

  /**
   * One root-relative path to an object.
   *
   * @param rootInstanceId the root the path starts from
   * @param rootSymbolId the qualified name the root was instantiated as; absent when the name no
   *     longer binds it and {@code #<id>} addresses it
   * @param path the steps as an object path ({@code "powertrain.engine.fuelIn"},
   *     {@code "wheels[2].hub"}); empty when the object is the root
   * @param steps the steps, one per feature traversed
   */
  public record Path(
      long rootInstanceId, Optional<String> rootSymbolId, String path, List<Step> steps) {
    /**
     * Creates a path, copying its steps.
     *
     * @param rootInstanceId the root
     * @param rootSymbolId the root's qualified name, when it still binds it
     * @param path the object path
     * @param steps the steps
     */
    public Path {
      Objects.requireNonNull(rootSymbolId, "rootSymbolId");
      Objects.requireNonNull(path, "path");
      steps = List.copyOf(steps);
    }
  }

  /**
   * One step of a path: a feature of the object before it, and the object the step reaches.
   *
   * @param featureName the holder's feature, a key of its {@link Instance#featureValues()}
   * @param featureSymbolId the FQN of the feature as declared
   * @param index the position among the values of a multi-valued feature, counted from 1 as an
   *     object path indexes them; 0 for a single-valued one
   * @param instanceId the object the step reaches
   */
  public record Step(String featureName, String featureSymbolId, int index, long instanceId) {
    /**
     * Creates a step.
     *
     * @param featureName the feature, never {@code null}
     * @param featureSymbolId the feature's FQN
     * @param index the position, 0 for single-valued
     * @param instanceId the object reached
     */
    public Step {
      Objects.requireNonNull(featureName, "featureName");
      Objects.requireNonNull(featureSymbolId, "featureSymbolId");
    }
  }
}
