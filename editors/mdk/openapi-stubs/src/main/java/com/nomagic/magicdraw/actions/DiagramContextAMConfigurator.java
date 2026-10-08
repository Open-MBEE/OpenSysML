// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.nomagic.magicdraw.actions;

import com.nomagic.actions.ActionsManager;
import com.nomagic.utils.PriorityProvider;
import com.nomagic.magicdraw.uml.symbols.DiagramPresentationElement;
import com.nomagic.magicdraw.uml.symbols.PresentationElement;

public interface DiagramContextAMConfigurator extends PriorityProvider {
  void configure(ActionsManager manager, DiagramPresentationElement diagram, PresentationElement[] selected, PresentationElement requestor);
}
