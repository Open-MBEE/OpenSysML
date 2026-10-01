package org.openmbee.opensysml.mdk.docgen;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.nomagic.magicdraw.plugins.Plugin;
import com.nomagic.magicdraw.plugins.PluginDescriptor;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class PluginLocatorTest {
  @Test
  void findsThePluginByIdAndCallsItsEntryPoint() {
    PluginLocator locator = new PluginLocator(() -> List.of(new Other(), new OpenSysML()));
    Map<String, Object> result = locator.run(new FakeElement("A"), "VERIFY", "");
    assertEquals("Part A", result.get("subject"));
    assertEquals("PASSED", result.get("status"));
  }

  @Test
  void explainsWhenThePluginIsMissing() {
    PluginLocator locator = new PluginLocator(() -> List.of(new Other()));
    IllegalStateException failure = assertThrows(IllegalStateException.class, locator::plugin);
    assertTrue(failure.getMessage().contains("org.openmbee.opensysml.mdk"), failure.getMessage());
    assertTrue(failure.getMessage().contains("not installed"), failure.getMessage());
  }

  @Test
  void explainsAnOutdatedPluginWithoutTheEntryPoint() {
    PluginLocator locator = new PluginLocator(() -> List.of(new Outdated()));
    IllegalStateException failure = assertThrows(IllegalStateException.class,
        () -> locator.run(new FakeElement("A"), "VERIFY", ""));
    assertTrue(failure.getMessage().contains("no docGen entry point"), failure.getMessage());
  }

  @Test
  void unwrapsRuntimeFailuresOfThePlugin() {
    PluginLocator locator = new PluginLocator(() -> List.of(new Failing()));
    IllegalArgumentException failure = assertThrows(IllegalArgumentException.class,
        () -> locator.run(new FakeElement("A"), "BOGUS", ""));
    assertEquals("unknown OpenSysML operation: BOGUS", failure.getMessage());
  }

  static class Described extends Plugin {
    private final String id;

    Described(String id) {
      this.id = id;
    }

    @Override
    public void init() {}

    @Override
    public boolean close() { return true; }

    @Override
    public boolean isSupported() { return true; }

    @Override
    public PluginDescriptor getDescriptor() {
      return new PluginDescriptor() {
        @Override
        public String getID() { return id; }
      };
    }
  }

  static final class Other extends Described {
    Other() { super("org.openmbee.mdk"); }
  }

  static final class Outdated extends Described {
    Outdated() { super(PluginLocator.PLUGIN_ID); }
  }

  public static final class OpenSysML extends Described {
    OpenSysML() { super(PluginLocator.PLUGIN_ID); }

    public Map<String, Object> docGen(Element element, String operation, String calcArguments) {
      return Flat.verify(element.getHumanName(), "PASSED");
    }
  }

  public static final class Failing extends Described {
    Failing() { super(PluginLocator.PLUGIN_ID); }

    public Map<String, Object> docGen(Element element, String operation, String calcArguments) {
      throw new IllegalArgumentException("unknown OpenSysML operation: " + operation);
    }
  }
}
