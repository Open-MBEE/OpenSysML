package org.openmbee.opensysml.mdk;

import com.nomagic.magicdraw.actions.ActionsConfiguratorsManager;
import com.nomagic.magicdraw.core.Project;
import com.nomagic.magicdraw.plugins.Plugin;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;
import java.nio.file.Path;
import java.util.Map;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.mdk.actions.OperationActions;
import org.openmbee.opensysml.mdk.bin.HostBinary;
import org.openmbee.opensysml.mdk.bridge.DocGenBridge;
import org.openmbee.opensysml.mdk.engine.Engine;

/**
 * Plugin entry point: registers the context-menu group; the service starts on the first run only.
 * {@link #docGen} is the surface the MDK DocGen extension reaches through reflection.
 */
public final class OpenSysMLPlugin extends Plugin {
  public static final String ID = "org.openmbee.opensysml.mdk";

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

  /** Runs one operation for MDK DocGen; see {@link DocGenBridge#run}. */
  public Map<String, Object> docGen(Element element, String operation, String calcArguments) {
    return new DocGenBridge(this::engine).run(Project.getProject(element), element, operation, calcArguments);
  }

  private synchronized Engine engine() {
    if (engine == null) engine = new Engine(getDescriptor().getPluginDirectory().toPath());
    return engine;
  }
}
