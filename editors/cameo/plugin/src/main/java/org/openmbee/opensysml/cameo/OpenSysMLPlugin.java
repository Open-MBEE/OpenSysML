package org.openmbee.opensysml.cameo;

import com.nomagic.magicdraw.actions.ActionsConfiguratorsManager;
import com.nomagic.magicdraw.plugins.Plugin;
import java.nio.file.Path;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.cameo.actions.OperationActions;
import org.openmbee.opensysml.cameo.bin.HostBinary;
import org.openmbee.opensysml.cameo.engine.Engine;

/** Plugin entry point: registers the context-menu group; the service starts on the first run only. */
public final class OpenSysMLPlugin extends Plugin {
  private Engine engine;

  @Override
  public void init() {
    OperationActions actions = new OperationActions(this::engine);
    ActionsConfiguratorsManager manager = ActionsConfiguratorsManager.getInstance();
    manager.addContainmentBrowserContextConfigurator(actions);
    manager.addBaseDiagramContextConfigurator("*", actions);
  }

  @Override
  public boolean close() {
    synchronized (this) {
      if (engine != null) engine.close();
      engine = null;
    }
    Connection.stopSharedServices();
    return true;
  }

  @Override
  public boolean isSupported() {
    Path pluginDirectory = getDescriptor().getPluginDirectory().toPath();
    return HostBinary.exists(pluginDirectory);
  }

  private synchronized Engine engine() {
    if (engine == null) engine = new Engine(getDescriptor().getPluginDirectory().toPath());
    return engine;
  }
}
