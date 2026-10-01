package org.openmbee.opensysml.mdk.model;

public interface ModelElement {
  String id();
  String qualifiedName();
  String humanName();
  Object handle();
}
