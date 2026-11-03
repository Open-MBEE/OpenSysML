package org.openmbee.mdk.magic.engine;

@FunctionalInterface
public interface Cancellation {
  boolean requested();
}
