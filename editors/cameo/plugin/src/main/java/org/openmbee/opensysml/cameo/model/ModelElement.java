package org.openmbee.opensysml.cameo.model;

public interface ModelElement {
  String id();
  String qualifiedName();
  String humanName();
  Object handle();
}
