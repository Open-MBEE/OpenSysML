// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.nomagic.magicdraw.plugins;

public abstract class Plugin {
  public abstract void init();
  public abstract boolean close();
  public abstract boolean isSupported();
  public PluginDescriptor getDescriptor() { throw new UnsupportedOperationException("compile-only stub"); }
}
