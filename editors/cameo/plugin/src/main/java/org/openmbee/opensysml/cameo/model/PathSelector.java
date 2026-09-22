package org.openmbee.opensysml.cameo.model;

public final class PathSelector {
  private static final String SERVICE =
      "com.dassault_systemes.modeler.magic.sysml.textual.SysMLTextualNotationService";

  private PathSelector() {}

  public static ModelPath select(boolean textualServiceOnClasspath, boolean selectionIsV2Element) {
    return textualServiceOnClasspath && selectionIsV2Element
        ? ModelPath.V2_TEXTUAL
        : ModelPath.V1_MDZIP;
  }

  public static boolean textualServiceOnClasspath() {
    try {
      Class.forName(SERVICE, false, PathSelector.class.getClassLoader());
      return true;
    } catch (ClassNotFoundException | LinkageError absent) {
      return false;
    }
  }
}
