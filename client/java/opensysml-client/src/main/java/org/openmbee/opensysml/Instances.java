package org.openmbee.opensysml;

import java.util.List;
import java.util.Optional;

/** Lookup shared by the results that carry the objects their verdicts and values refer to. */
final class Instances {

  private Instances() {}

  static Optional<Instance> find(List<Instance> instances, long instanceId) {
    for (Instance instance : instances) {
      if (instance.id() == instanceId) {
        return Optional.of(instance);
      }
    }
    return Optional.empty();
  }
}
