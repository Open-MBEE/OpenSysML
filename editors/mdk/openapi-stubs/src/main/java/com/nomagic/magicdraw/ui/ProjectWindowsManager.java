// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.nomagic.magicdraw.ui;

import com.nomagic.magicdraw.core.Project;

public interface ProjectWindowsManager extends WindowsManager {
  void addWindow(Project project, ProjectWindow projectWindow);
  void activateWindow(Project project, String id);
  void removeWindow(Project project, String id);
}
