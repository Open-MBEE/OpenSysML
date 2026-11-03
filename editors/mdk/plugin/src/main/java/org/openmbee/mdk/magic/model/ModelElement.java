package org.openmbee.mdk.magic.model;

public interface ModelElement {
  String id();
  String qualifiedName();
  String humanName();
  Object handle();
}
