// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.nomagic.magicdraw.actions;

import com.nomagic.actions.ActionsManager;
import com.nomagic.utils.PriorityProvider;
import com.nomagic.magicdraw.ui.browser.Tree;

public interface BrowserContextAMConfigurator extends PriorityProvider {
  void configure(ActionsManager manager, Tree browser);
}
