package org.openmbee.opensysml.mdk.docgen;

import com.nomagic.magicdraw.uml.BaseElement;
import java.util.Map;

/**
 * Runs one OpenSysML operation on a Cameo element, UML or SysML v2 (both are {@link BaseElement});
 * the plugin behind it is found at call time.
 */
@FunctionalInterface
public interface BridgeRunner {
  Map<String, Object> run(BaseElement element, String operation, String calcArguments);
}
