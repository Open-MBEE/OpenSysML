package org.openmbee.mdk.magic.docgen;

import com.nomagic.magicdraw.uml.BaseElement;
import java.util.Map;

/**
 * Runs one OpenSysML operation on a Cameo element, UML or SysML v2 (both are {@link BaseElement});
 * the plugin behind it is found at call time.
 */
@FunctionalInterface
public interface BridgeRunner {
  Map<String, Object> run(BaseElement element, String operation, String calcArguments);

  /**
   * Runs as the three-argument {@code run} does, a query's answers also written as {@code
   * QVTS::QueryResult} views when {@code view} is {@code text} or {@code rendered}; blank asks none.
   */
  default Map<String, Object> run(BaseElement element, String operation, String calcArguments, String view) {
    if (view == null || view.isBlank()) return run(element, operation, calcArguments);
    throw new IllegalStateException("this runner cannot write a query's answer as a view");
  }
}
