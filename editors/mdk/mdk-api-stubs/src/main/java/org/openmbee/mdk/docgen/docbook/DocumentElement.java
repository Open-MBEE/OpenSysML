// Compile-only stub of the OpenMBEE MDK API the bridge uses; never shipped.
package org.openmbee.mdk.docgen.docbook;

import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;

public abstract class DocumentElement {
  private String id;
  private String title;
  private Element from;

  public void setId(String id) { this.id = id; }
  public String getId() { return id; }
  public void setTitle(String title) { this.title = title; }
  public String getTitle() { return title; }
  public Element getFrom() { return from; }
  public void setFrom(Element from) { this.from = from; }
}
