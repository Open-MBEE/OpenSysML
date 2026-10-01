// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.nomagic.uml2.ext.jmi.helpers;

import com.nomagic.magicdraw.core.Project;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Package;
import com.nomagic.uml2.ext.magicdraw.mdprofiles.Profile;
import com.nomagic.uml2.ext.magicdraw.mdprofiles.Stereotype;
import java.util.Collection;

public class StereotypesHelper {
  public static Object getStereotypePropertyFirst(Element element, String stereotypeName, String propertyName) {
    throw new UnsupportedOperationException("compile-only stub");
  }
  public static Profile getProfile(Project project, String profileName) {
    throw new UnsupportedOperationException("compile-only stub");
  }
  public static Stereotype getStereotype(Project project, String stereotypeName, Profile profile) {
    throw new UnsupportedOperationException("compile-only stub");
  }
  public static Stereotype createStereotype(
      Element owner, String name, Collection<com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Class> metaClasses) {
    throw new UnsupportedOperationException("compile-only stub");
  }
  public static Collection<Profile> getAppliedProfiles(Package pkg) {
    throw new UnsupportedOperationException("compile-only stub");
  }

  public static void applyProfile(Package pkg, Profile profile) {
    throw new UnsupportedOperationException("compile-only stub");
  }

  public static com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Class getMetaClassByName(Project project, String name) {
    throw new UnsupportedOperationException("compile-only stub");
  }
}
