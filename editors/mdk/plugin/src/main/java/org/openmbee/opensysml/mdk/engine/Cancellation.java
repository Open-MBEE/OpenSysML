package org.openmbee.opensysml.mdk.engine;

@FunctionalInterface
public interface Cancellation {
  boolean requested();
}
