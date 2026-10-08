// Compile-only stub of the OpenMBEE MDK API the bridge uses; never shipped.
package org.openmbee.mdk.docgen.docbook;

public class DBText extends DocumentElement {
  private Object text;

  public DBText() {}
  public DBText(Object text) { this.text = text; }
  public void setText(Object text) { this.text = text; }
  public Object getText() { return text; }
}
