package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * What {@link Model#instantiate(String)} built: the object asked for, and every object reachable
 * from it, so an {@link Value.InstanceReference} resolves without a further call.
 *
 * @param root the object of the symbol that was instantiated
 * @param reachable every object built, including {@code root}, in the order the service reported
 * @param diagnostics what the service reported while building it
 */
public record Instantiation(Instance root, List<Instance> reachable, List<Diagnostic> diagnostics) {

  /**
   * Creates an instantiation, copying its collections.
   *
   * @param root the object instantiated
   * @param reachable every object built
   * @param diagnostics diagnostics reported
   */
  public Instantiation {
    Objects.requireNonNull(root, "root");
    reachable = List.copyOf(reachable);
    diagnostics = List.copyOf(diagnostics);
  }

  /**
   * The object a value refers to.
   *
   * @param reference a reference the service reported
   * @return the referenced object, absent when it is not among the objects built
   */
  public Optional<Instance> resolve(Value.InstanceReference reference) {
    Objects.requireNonNull(reference, "reference");
    return instance(reference.instanceId());
  }

  /**
   * The object of an id.
   *
   * @param instanceId the id the service gave the object
   * @return the object, absent when it is not among the objects built
   */
  public Optional<Instance> instance(long instanceId) {
    if (root.id() == instanceId) {
      return Optional.of(root);
    }
    for (Instance instance : reachable) {
      if (instance.id() == instanceId) {
        return Optional.of(instance);
      }
    }
    return Optional.empty();
  }
}
