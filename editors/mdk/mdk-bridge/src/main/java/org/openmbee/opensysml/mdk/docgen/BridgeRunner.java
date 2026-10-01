package org.openmbee.opensysml.mdk.docgen;

import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;
import java.util.Map;

/** Runs one OpenSysML operation on a Cameo element; the plugin behind it is found at call time. */
@FunctionalInterface
public interface BridgeRunner {
  Map<String, Object> run(Element element, String operation, String calcArguments);
}
