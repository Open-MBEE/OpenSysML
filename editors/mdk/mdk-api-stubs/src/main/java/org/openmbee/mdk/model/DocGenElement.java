// Compile-only stub of the OpenMBEE MDK API the bridge uses; never shipped.
package org.openmbee.mdk.model;

import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;

public abstract class DocGenElement {
  protected boolean ignore;
  protected boolean loop;
  protected String titleSuffix;
  protected String titlePrefix;
  protected boolean useContextNameAsTitle;
  protected Element dgElement;

  public boolean getIgnore() { return ignore; }
  public void setIgnore(boolean ignore) { this.ignore = ignore; }
  public void setTitleSuffix(String titleSuffix) { this.titleSuffix = titleSuffix; }
  public void setTitlePrefix(String titlePrefix) { this.titlePrefix = titlePrefix; }
  public String getTitlePrefix() { return titlePrefix; }
  public String getTitleSuffix() { return titleSuffix; }
  public void setUseContextNameAsTitle(boolean useContextNameAsTitle) { this.useContextNameAsTitle = useContextNameAsTitle; }
  public boolean getUseContextNameAsTitle() { return useContextNameAsTitle; }
  public void setDgElement(Element dgElement) { this.dgElement = dgElement; }
  public Element getDgElement() { return dgElement; }
  public void setLoop(boolean loop) { this.loop = loop; }
  public boolean getLoop() { return loop; }
}
