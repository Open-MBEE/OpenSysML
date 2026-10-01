package org.openmbee.opensysml.mdk.docgen;

import com.nomagic.magicdraw.plugins.Plugin;
import com.nomagic.magicdraw.plugins.PluginUtils;
import com.nomagic.magicdraw.uml.BaseElement;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.util.Collection;
import java.util.Map;

/**
 * Reaches the OpenSysML MDK plugin through Cameo's plugin registry. This jar is loaded by MDK's
 * extensions classloader and the plugin by its own, so the call goes through reflection on a
 * method whose signature uses only Cameo and JDK types.
 */
public final class PluginLocator implements BridgeRunner {
  public static final String PLUGIN_ID = "org.openmbee.opensysml.mdk";
  static final String METHOD = "docGen";

  private final java.util.function.Supplier<Collection<Plugin>> plugins;

  public PluginLocator() {
    this(PluginUtils::getPlugins);
  }

  PluginLocator(java.util.function.Supplier<Collection<Plugin>> plugins) {
    this.plugins = plugins;
  }

  @Override
  public Map<String, Object> run(BaseElement element, String operation, String calcArguments) {
    Plugin plugin = plugin();
    Method method;
    try {
      method = plugin.getClass().getMethod(METHOD, BaseElement.class, String.class, String.class);
    } catch (NoSuchMethodException missing) {
      throw new IllegalStateException(
          "the installed OpenSysML MDK plugin has no " + METHOD + " entry point; update it to match this extension",
          missing);
    }
    Object result;
    try {
      result = method.invoke(plugin, element, operation, calcArguments);
    } catch (IllegalAccessException inaccessible) {
      throw new IllegalStateException(METHOD + " is not accessible on the OpenSysML MDK plugin", inaccessible);
    } catch (InvocationTargetException failed) {
      Throwable cause = failed.getCause();
      if (cause instanceof RuntimeException runtime) throw runtime;
      if (cause instanceof Error error) throw error;
      throw new IllegalStateException(cause);
    }
    if (!(result instanceof Map<?, ?> map)) {
      throw new IllegalStateException(METHOD + " returned " + (result == null ? "null" : result.getClass().getName()));
    }
    @SuppressWarnings("unchecked")
    Map<String, Object> typed = (Map<String, Object>) map;
    return typed;
  }

  Plugin plugin() {
    for (Plugin candidate : plugins.get()) {
      if (candidate.getDescriptor() != null && PLUGIN_ID.equals(candidate.getDescriptor().getID())) {
        return candidate;
      }
    }
    throw new IllegalStateException(
        "the OpenSysML MDK plugin (" + PLUGIN_ID + ") is not installed or did not load; install it from the same zip as this extension");
  }
}
