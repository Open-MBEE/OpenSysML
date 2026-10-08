// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.nomagic.magicdraw.uml;

import com.nomagic.magicdraw.core.Project;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;

public class Finder {
  public static ByQualifiedNameFinder byQualifiedName() { throw new UnsupportedOperationException("compile-only stub"); }

  public static class ByQualifiedNameFinder {
    public <T extends Element> T find(Project project, String qualifiedName) {
      throw new UnsupportedOperationException("compile-only stub");
    }
  }
}
