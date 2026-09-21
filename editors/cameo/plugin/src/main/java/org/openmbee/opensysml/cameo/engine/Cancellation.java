package org.openmbee.opensysml.cameo.engine;

@FunctionalInterface
public interface Cancellation {
  boolean requested();
}
