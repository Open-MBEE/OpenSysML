// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.nomagic.utils;

@FunctionalInterface
public interface PriorityProvider {
  int HIGH_PRIORITY = 0;
  int MEDIUM_PRIORITY = 1;
  int LOW_PRIORITY = 2;

  int getPriority();
}
